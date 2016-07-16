This assumes a setup with two directories: src and dest.
Assuming the two directories are in sync (identical files) to begin with.
This directory will copy to dest all the modifications happening in src: files
added, modified or removed.

Usage:
$ brew update
$ brew install watchman
$ go build .
$ ./replicate_dir_watchman folder1 folder2
