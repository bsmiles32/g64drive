package pi

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func toChar(b byte) byte {
	if b < 32 || b > 126 {
		return '.'
	}
	return b
}

// returns an output similar to hexdump -C
// It differs from encoding/hex.Dump mainly it 2 aspects (apart from being a naive implementation):
// * will pad data to a multiple of 16 bytes
// * will squeeze consecutive equal lines and print a * instead (very useful for sparse data like flash)
func hexDump(data []byte) string {
	// Pad data to a multiple of 16 to ease dumping logic
	for len(data)%16 != 0 {
		data = append(data, byte(0))
	}

	var b strings.Builder

	printStar := true
	for k := 0; k < len(data); k += 16 {
		if k >= 16 && bytes.Equal(data[k:k+16], data[k-16:k]) {
			if printStar {
				fmt.Fprintf(&b, "*\n")
				printStar = false
			}
			continue
		}

		printStar = true
		fmt.Fprintf(&b, "%08x  %02x %02x %02x %02x %02x %02x %02x %02x  %02x %02x %02x %02x %02x %02x %02x %02x  |%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s%s|\n", k,
			data[k+0], data[k+1], data[k+2], data[k+3], data[k+4], data[k+5], data[k+6], data[k+7],
			data[k+8], data[k+9], data[k+10], data[k+11], data[k+12], data[k+13], data[k+14], data[k+15],
			string(toChar(data[k+0])), string(toChar(data[k+1])), string(toChar(data[k+2])), string(toChar(data[k+3])),
			string(toChar(data[k+4])), string(toChar(data[k+5])), string(toChar(data[k+6])), string(toChar(data[k+7])),
			string(toChar(data[k+8])), string(toChar(data[k+9])), string(toChar(data[k+10])), string(toChar(data[k+11])),
			string(toChar(data[k+12])), string(toChar(data[k+13])), string(toChar(data[k+14])), string(toChar(data[k+15])),
		)
	}

	return b.String()
}

func TestPiRead(t *testing.T) {

	type ReadBurstInfo struct {
		address Address
		size    int
	}

	testCases := []struct {
		address       Address
		size          int
		pageBits      int
		expectedReads []ReadBurstInfo
	}{
		{address: 1, size: 0, pageBits: 9, expectedReads: []ReadBurstInfo{}},
		{address: 0, size: 1, pageBits: 9, expectedReads: []ReadBurstInfo{
			{address: 0, size: 4},
		},
		},
		{address: 1, size: 1, pageBits: 9, expectedReads: []ReadBurstInfo{
			{address: 0, size: 4},
		},
		},
		{address: 2, size: 1, pageBits: 9, expectedReads: []ReadBurstInfo{
			{address: 0, size: 4},
		},
		},
		{address: 3, size: 1, pageBits: 9, expectedReads: []ReadBurstInfo{
			{address: 0, size: 4},
		},
		},
		{address: 4, size: 1, pageBits: 9, expectedReads: []ReadBurstInfo{
			{address: 4, size: 4},
		},
		},
		{address: 0, size: 0x00100000, pageBits: 20, expectedReads: []ReadBurstInfo{
			{address: 0, size: 0x00100000},
		},
		},
		{address: 0, size: 2048, pageBits: 9, expectedReads: []ReadBurstInfo{
			{address: 0, size: 512},
			{address: 512, size: 512},
			{address: 1024, size: 512},
			{address: 1536, size: 512},
		},
		},
		{address: 3, size: 523, pageBits: 9, expectedReads: []ReadBurstInfo{
			{address: 0, size: 512},
			{address: 512, size: 16},
		},
		},
		{address: 508, size: 16, pageBits: 9, expectedReads: []ReadBurstInfo{
			{address: 508, size: 4},
			{address: 512, size: 12},
		},
		},
		{address: 0x08007ffe, size: 16, pageBits: 15, expectedReads: []ReadBurstInfo{
			{address: 0x08007ffc, size: 4},
			{address: 0x08008000, size: 16},
		},
		},
		{address: 0x08000000, size: 0x00020000, pageBits: 15, expectedReads: []ReadBurstInfo{
			{address: 0x08000000, size: 0x8000},
			{address: 0x08008000, size: 0x8000},
			{address: 0x08010000, size: 0x8000},
			{address: 0x08018000, size: 0x8000},
		},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("address=%08x size=%08x pageBits=%d", tc.address, tc.size, tc.pageBits), func(t *testing.T) {
			reads := make([]ReadBurstInfo, 0)
			got := make([]byte, tc.size)
			err := Read(context.Background(), got, tc.address, tc.pageBits, BurstReaderAtFunc(func(data []byte, address Address) error {
				reads = append(reads, ReadBurstInfo{address, len(data)})
				for k := 0; k < len(data); k++ {
					data[k] = byte(address + Address(k))
				}

				return nil
			}))

			if err != nil {
				t.Fatal(err)
			}

			// Checking expected reads
			//t.Logf("reads: %+v", reads)
			if len(tc.expectedReads) != len(reads) {
				t.Errorf("unexpected reads: %+v != %+v", tc.expectedReads, reads)
			} else {

				for i := 0; i < len(reads); i++ {
					if expected, got := tc.expectedReads[i], reads[i]; expected != got {
						t.Errorf("unexpected reads (%d): %+v != %+v", i, expected, got)
					}
				}
			}

			// Checking content
			for k, v := range got {
				expected := byte(tc.address + Address(k))
				if expected != v {
					t.Errorf("unexpected value: %08x != %08x", expected, v)
					break
				}
			}

			if t.Failed() {
				t.Log(hexDump(got))
			}
		})
	}
}

func TestSplitBurst(t *testing.T) {

	type BurstInfo struct {
		address Address
		size    int
	}

	testCases := []struct {
		address        Address
		size           int
		pageBits       int
		expectedBursts []BurstInfo
	}{
		{address: 0, size: 0, pageBits: 9, expectedBursts: []BurstInfo{}},
		{address: 0, size: 4, pageBits: 9, expectedBursts: []BurstInfo{
			{address: 0, size: 4},
		},
		},
		{address: 4, size: 8, pageBits: 9, expectedBursts: []BurstInfo{
			{address: 4, size: 8},
		},
		},
		{address: 0, size: 0x00100000, pageBits: 20, expectedBursts: []BurstInfo{
			{address: 0, size: 0x00100000},
		},
		},
		{address: 0, size: 2048, pageBits: 9, expectedBursts: []BurstInfo{
			{address: 0, size: 512},
			{address: 512, size: 512},
			{address: 1024, size: 512},
			{address: 1536, size: 512},
		},
		},
		{address: 4, size: 524, pageBits: 9, expectedBursts: []BurstInfo{
			{address: 4, size: 508},
			{address: 512, size: 16},
		},
		},
		{address: 508, size: 16, pageBits: 9, expectedBursts: []BurstInfo{
			{address: 508, size: 4},
			{address: 512, size: 12},
		},
		},
		{address: 0x08007ffc, size: 16, pageBits: 15, expectedBursts: []BurstInfo{
			{address: 0x08007ffc, size: 4},
			{address: 0x08008000, size: 12},
		},
		},
		{address: 0x08000000, size: 0x00020000, pageBits: 15, expectedBursts: []BurstInfo{
			{address: 0x08000000, size: 0x8000},
			{address: 0x08008000, size: 0x8000},
			{address: 0x08010000, size: 0x8000},
			{address: 0x08018000, size: 0x8000},
		},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("address=%08x size=%08x pageBits=%d", tc.address, tc.size, tc.pageBits), func(t *testing.T) {
			bursts := make([]BurstInfo, 0)
			got := make([]byte, tc.size)
			n, err := SplitBursts(context.Background(), got, tc.address, tc.pageBits, func(data []byte, address Address) error {
				bursts = append(bursts, BurstInfo{address, len(data)})
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			if n != len(got) {
				t.Errorf("unexpected n: %d != %d", len(got), n)
			}

			if len(tc.expectedBursts) != len(bursts) {
				t.Errorf("unexpected bursts: %+v != %+v", tc.expectedBursts, bursts)
			} else {
				for i := 0; i < len(bursts); i++ {
					if expected, got := tc.expectedBursts[i], bursts[i]; expected != got {
						t.Errorf("unexpected bursts (%d): %+v != %+v", i, expected, got)
					}
				}
			}
		})
	}
}
