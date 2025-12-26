package main

import (
	"bytes"
	"time"
)

// getString finds the null-terminated C-string, but goes up to some absolute max limit.
func getString(buf *bytes.Reader, maxChars int) string {
	myString := make([]byte, 0, 256)

	for len(myString) < maxChars {
		c := readUint8(buf)
		if c == 0 {
			return string(myString)
		}
		myString = append(myString, c)
	}

	panic("exceeded max string size!")
}

func readUint8(buf *bytes.Reader) uint8 {
	val, err := buf.ReadByte()
	if err != nil {
		panic(err)
	}
	return val
}

func readUint32(buf *bytes.Reader) uint32 {
	var a uint32

	a = uint32(readUint8(buf))
	a |= uint32(readUint8(buf)) << 8
	a |= uint32(readUint8(buf)) << 16
	a |= uint32(readUint8(buf)) << 24

	return a
}

func readUint16(buf *bytes.Reader) uint16 {
	var a uint16

	a = uint16(readUint8(buf))
	a |= uint16(readUint8(buf)) << 8

	return a
}

func readUint8Block(buf *bytes.Reader, size int) []uint8 {
	buffer := make([]uint8, size)
	for i := 0; i < size; i++ {
		buffer[i] = readUint8(buf)
	}
	return buffer
}

func readUint16Block(buf *bytes.Reader, size int) []uint16 {
	buffer := make([]uint16, size)
	for i := 0; i < size; i++ {
		buffer[i] = readUint16(buf)
	}
	return buffer
}

func readTag(buf *bytes.Reader, tag string) {
	buffer := readUint8Block(buf, len(tag))
	if string(buffer) != tag {
		panic("expected tag: " + tag + " in resource")
	}
}

func peekUint16(data []byte, offset *uint32) uint16 {
	var result uint16

	result = uint16(data[int(*offset)])
	*offset += 1
	result |= uint16(data[int(*offset)]) << 8
	*offset += 1

	return result
}

func peekUint16Block(data []byte, offset *uint32, dest []uint16, length int) {
	for i := 0; i < length; i++ {
		dest[i] = peekUint16(data, offset)
	}
}

func getDayOfYear() int {
	return time.Now().YearDay() - 1
}

func getHour() int {
	return time.Now().Hour()
}

func getMonthAndDay() (int, int) {
	t := time.Now()
	return int(t.Month()), t.Day()
}
