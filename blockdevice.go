// +build !linux,!darwin,!freebsd !cgo

package main

import (
	"log"
	"os"
)

func getBlockDeviceSize(f *os.File) int64 {
	log.Println("not supported")
	return 0
}
