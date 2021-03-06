/*
forganizer is an utility that organizes files into foldres according to file modification date.
Can skip files newer than defined amount of days. If a target file with the same content exists it removes the source file.
In case files are different it renames source file by adding _X suffix.

Use case:
You have many photos in your phone camera folder.
You run `forganizer -r -d 30 /phone/camera /desktop/photoarchive/` and it moves all files into /Year/Month directory structure
leaving let's say 30 last days of photos on your phone.
*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/barasher/go-exiftool"
	"github.com/codingsince1985/checksum"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type options struct {
	exif       bool
	recursive  bool
	dry_run    bool
	days_older int
}

func main() {
	var opts options
	flag.BoolVar(&opts.exif, "exif", false, "read date from EXIF data if possible")
	flag.BoolVar(&opts.recursive, "r", false, "recursive into directories")
	flag.BoolVar(&opts.dry_run, "dry", false, "dry run, do not modify files or directories, only print results")
	flag.IntVar(&opts.days_older, "d", 0, "only process files older than this number of days")
	flag.Parse()
	src, dst := flag.Arg(0), flag.Arg(1)
	if src == "" || dst == "" {
		fmt.Println("Error: SRC or DST directories not set")
		printUsage()
		return
	}
	processDir(src, dst, opts)
}

var et *exiftool.Exiftool

func processDir(src string, dst string, opts options) {
	fmt.Printf("Processing directory: %v\n", src)
	dir, err := os.Open(src)
	if err != nil {
		fmt.Printf("Error accessing directory: %v\n", err)
		return
	}
	if opts.exif {
		et, err = exiftool.NewExiftool()
		if err != nil {
			fmt.Printf("Error when intializing EXIF: %v\n", err)
			return
		}
		defer et.Close()
	}
	for {
		files, err := dir.Readdir(100)
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Printf("Error listing directory %v: %v\n", src, err)
			return
		}
		keep_after := time.Now().AddDate(0, 0, -1*opts.days_older)
		for i := 0; i < len(files); i++ {
			if files[i].IsDir() {
				if opts.recursive {
					defer processDir(path.Join(src, files[i].Name()), dst, opts)
				}
				continue
			}
			mod_time := files[i].ModTime()
			if mod_time.After(keep_after) {
				fmt.Println("  Skipping file ", files[i].Name(), ": is too new ", files[i].ModTime())
				continue
			}
			fmt.Printf("  Processing file: %v\n", files[i].Name())
			processFile(src, files[i], dst, opts)
		}
	}
}

func processFile(src_dir string, source os.FileInfo, dst_dir string, opts options) {
	var mod_time time.Time
	var err error
	name := source.Name()
	source_path := path.Join(src_dir, name)
	if opts.exif {
		mod_time, err = getExifTime(source_path)
		if err != nil {
			fmt.Println("    Exif error:", err)
			mod_time = source.ModTime()
		}
	} else {
		mod_time = source.ModTime()
	}
	target_dir := path.Join(
		dst_dir,
		fmt.Sprintf("%d/%02d", mod_time.Year(), mod_time.Month()))
	target_path := path.Join(target_dir, name)
	fmt.Print("    -> ", target_path, ": ")
	if is_exists, target := isExists(target_path); is_exists {
		if os.SameFile(source, target) {
			fmt.Println("same file, skipping")
			return
		}
		if haveSameContents(source_path, target_path) {
			fmt.Print("same contents, ")
			if !opts.dry_run {
				err := os.Remove(source_path)
				if err != nil {
					fmt.Println("error removing: ", err)
					return
				}
			}
			fmt.Println("source removed")
			return
		}
		target_path = genUniqueName(target_dir, name)
		fmt.Print("different file exists, moving to -> ", target_path, ": ")
	}
	if !opts.dry_run {
		if exists, _ := isExists(target_dir); !exists {
			_, src_dir_info := isExists(src_dir)
			err := os.MkdirAll(target_dir, src_dir_info.Mode())
			if err != nil {
				fmt.Println("error creating target directory: ", err)
				return
			}
		}
		err := os.Rename(source_path, target_path)
		if err != nil {
			fmt.Println("error: ", err)
			return
		}
	}
	fmt.Println("moved")
}

func isExists(filename string) (bool, os.FileInfo) {
	fileinfo, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, fileinfo
}

func genUniqueName(dir, filename string) string {
	split := strings.Split(filename, ".")
	var name, ext string
	switch len(split) {
	case 1:
		name, ext = split[0], ""
	case 2:
		name, ext = split[0], split[1]
	default:
		name = strings.Join(split[0:len(split)-1], ".")
		ext = split[len(split)-1]
	}
	for i := 1; i > 0; i++ {
		newpath := path.Join(dir, fmt.Sprintf("%s_%d.%s", name, i, ext))
		if is_exists, _ := isExists(newpath); !is_exists {
			return newpath
		}
	}
	return ""
}

func haveSameContents(file1, file2 string) bool {
	md5_1, _ := checksum.MD5sum(file1)
	md5_2, _ := checksum.MD5sum(file2)
	return md5_1 == md5_2
}

func toString(v interface{}) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func getExifTime(path string) (time.Time, error) {
	var (
		t time.Time
	)
	fileInfo := et.ExtractMetadata(path)[0]
	if fileInfo.Err != nil {
		return time.Now(), fileInfo.Err
	}
	v, ok := fileInfo.Fields["DateTimeOriginal"]
	if !ok {
		v, ok = fileInfo.Fields["CreateDate"]
	}
	if !ok {
		v, ok = fileInfo.Fields["ModifyDate"]
	}
	if !ok {
		return t, errors.New("No EXIF date found")
	}
	date_str := toString(v)
	if len(date_str) > 19 {
		date_str = date_str[:19]
	}
	t, err := time.Parse("2006:01:02 15:04:05", date_str)
	if err != nil {
		return t, errors.New("Error parsing time")
	}
	return t, nil
}

func printUsage() {
	fmt.Println(`
Usage: forganize [-r] [-dry] [-exif] [-d DAYS] SRC DST

SRC - source directory
DST - root directory for organized files

Options:
    -r - scan files recursively into SRC subdirectories
    -d DAYS - do not process files newer than DAYS days from now
    -exif - use EXIF date if possible (needs installed exiftool package)
    -dry - dry run
`)
}
