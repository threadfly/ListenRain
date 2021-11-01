package main

import "unsafe"

func StringToBytes(s string) []byte {
	sp := (*[2]uintptr)(unsafe.Pointer(&s))
	btp := [3]uintptr{sp[0], sp[1], sp[1]}
	return *(*[]byte)(unsafe.Pointer(&btp))
}

func BytesToString(bytes []byte) string {
	sp := (*[3]uintptr)(unsafe.Pointer(&bytes))
	btp := [2]uintptr{sp[0], sp[1]}
	return *(*string)(unsafe.Pointer(&btp))
}

func ResetBytes(bytes []byte, resetSize int) []byte {
	sp := (*[3]uintptr)(unsafe.Pointer(&bytes))
	sp[0] -= uintptr(resetSize)
	sp[1] += uintptr(resetSize)
	sp[2] += uintptr(resetSize)
	btp := [3]uintptr{sp[0], sp[1], sp[2]}
	return *(*[]byte)(unsafe.Pointer(&btp))
}
