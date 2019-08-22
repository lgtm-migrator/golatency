// +build cgo

package main

import (
	"os"
)

// #include <sys/disk.h>
// #include <sys/ioctl.h>
//
// unsigned long long get_size(int fd) {
//	unsigned long long blocksize = 0;
//	unsigned long long blockcount = 0;
//
// 	ioctl(fd, DKIOCGETBLOCKSIZE, &blocksize);
// 	ioctl(fd, DKIOCGETBLOCKCOUNT, &blockcount);
//  return blockcount*blocksize;
// }
import "C"

func getBlockDeviceSize(f *os.File) int64 {
	size := int64(C.get_size(C.int(f.Fd())))
	return size
}
