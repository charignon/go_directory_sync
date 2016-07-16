package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
)

func getSockName() string {
	out, err := exec.Command("/usr/local/bin/watchman", "get-sockname").Output()
	if err != nil {
		log.Panic(err)
	}
	var dat map[string]interface{}
	if err := json.Unmarshal(out, &dat); err != nil {
		log.Panic(err)
	}
	return dat["sockname"].(string)
}

func getConnection(sockpath string) net.Conn {
	conn, err := net.Dial("unix", sockpath)
	if err != nil {
		log.Panic(err)
	}
	return conn
}

func watchproject(conn net.Conn, project string) {
	fmt.Fprintf(conn, "[\"watch-project\", \"%s\"]\n", project)
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		log.Panic(err)
	}
	fmt.Println(status)
}

func copy(dst, src string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return
	}
	err = out.Close()
	return
}

func getEvent(conn net.Conn) ([]string, bool, error) {
	out, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return nil, false, nil
	}

	var dat map[string]interface{}
	fmt.Println(out)
	if err := json.Unmarshal([]byte(out), &dat); err != nil {
		return nil, false, nil
	}
	_, hasfiles := dat["files"]
	freshinstance, okfreshinstance := dat["is_fresh_instance"]
	var isfreshinstance bool
	if okfreshinstance {
		isfreshinstance = freshinstance.(bool)
	}
	ret := []string{}
	if hasfiles {
		files := dat["files"].([]interface{})
		for _, l := range files {
			ret = append(ret, l.(string))
		}
	}
	fmt.Println(ret, isfreshinstance)
	return ret, isfreshinstance, nil
}

// From https://stackoverflow.com/questions/30697324/how-to-check-if-directory-on-path-is-empty
func IsEmpty(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true
	}
	return false // Either not empty or error, suits both cases
}

func removeEmptyFolder(fpath string) {
	if IsEmpty(fpath) {
		os.Remove(fpath)
		dir, _ := path.Split(fpath)
		removeEmptyFolder(dir)
	}
	return
}

func subscribe(conn net.Conn, project string, dest string) {
	fmt.Fprintf(conn, "[\"subscribe\", \"%s\", \"gosub\", {\"expression\":[\"allof\", [\"type\", \"f\", \"d\"]],\"fields\": [\"name\"]}]\n", project)
	for {
		listoffiles, freshinstance, err := getEvent(conn)
		if err != nil {
			log.Panic(err)
		}
		if !freshinstance {
			for _, f := range listoffiles {
				src := path.Join(project, f)
				dst := path.Join(dest, f)
				// Is it a removal?
				var remove bool
				if _, err := os.Stat(src); os.IsNotExist(err) {
					remove = true
				}
				dir, _ := path.Split(dst)
				if remove {
					log.Println("Trying to remove ", src, dst)
					os.Remove(dst)
					// Remove empty folders too
					// removeEmptyFolder(dir)
				} else {
					log.Println("Trying to copy ", src, dst)
					if _, err := os.Stat(dir); os.IsNotExist(err) {
						os.MkdirAll(dir, 0777)
					}
					// Is it a dir?
					i, _ := os.Stat(src)
					if i.IsDir() {
						os.Mkdir(dst, i.Mode().Perm())
					} else {
						err = copy(dst, src)
						if err != nil {
							log.Println(err)
						}
					}
				}
			}
		}
	}
}

func listRecursiveFiles(path string) ([]string, error) {
	fileList := []string{}
	var toperr error
	filepath.Walk(path, func(p string, f os.FileInfo, err error) error {
		if err != nil {
			toperr = err
			return nil
		}
		if !f.IsDir() {
			fileList = append(fileList, p)
		}
		return nil
	})
	sort.Strings(fileList)
	return fileList, toperr
}

func computeHash(content []byte) string {
	h := sha1.New()
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func validateIdentical(src string, dst string) {
	f1, err1 := listRecursiveFiles(src)
	f2, err2 := listRecursiveFiles(dst)
	if err1 != nil {
		log.Fatal(err1)
	}
	if err2 != nil {
		log.Fatal(err2)
	}
	if len(f1) != len(f2) {
		log.Fatal("Folders are not identical")
	}
	for i, f := range f1 {
		log.Println("Checking:", f)
		other := f2[i]
		c1, err1 := ioutil.ReadFile(f)
		c2, err1 := ioutil.ReadFile(other)
		if err1 != nil {
			log.Fatal(err1)
		}
		if err2 != nil {
			log.Fatal(err2)
		}
		if computeHash(c1) != computeHash(c2) {
			log.Fatal("Folders are not identical")
		}
	}
}

func main() {

	if len(os.Args) != 3 {
		log.Fatal("Expecting 2 arguments <folder to copy> <folder to copy to>")
	}
	src := os.Args[1]
	dest := os.Args[2]
	validateIdentical(src, dest)
	sockpath := getSockName()
	conn := getConnection(sockpath)
	watchproject(conn, src)
	subscribe(conn, src, dest)
}
