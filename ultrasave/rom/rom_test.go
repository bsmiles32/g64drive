package rom

import (
	"bytes"
	"fmt"
	"github.com/rasky/g64drive/drive64"
	"github.com/rasky/g64drive/ultrasave"
	"github.com/rasky/g64drive/ultrasave/pi"
	"strings"
	"testing"
)

func setup(t *testing.T, opts ...RomOption) *Rom {
	t.Helper()

	dev, err := drive64.NewDeviceSingle()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		dev.Close()
	})

	// Check firmware version and verify if it's new enough
	if _, fwver, _, err := dev.CmdVersionRequest(); err == nil {
		if fwver < 203 {
			t.Skipf("requires 64drive firmware >= 2.03, found: %v\nDownload a newer firmware from http://64drive.retroactive.be, and then run \"g64drive firmware upgrade\" to upgrade", fwver)
		}
	}

	err = dev.CmdStandAloneEnter()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		dev.CmdStandAloneLeave()
	})

	udev := ultrasave.New64DriveAdapters(dev)
	if udev == nil {
		t.Skip("unable to get 64drive adapter")
	}


	t.Logf("udev type: %T", udev)

	c, ok := udev.(Controller)
	if !ok {
		t.Skip("adapter doesn't support Controller interface")
	}

	r, err := New(c, opts...)
	if err != nil {
		t.Fatal(err)
	}

	return r
}

func (r *Rom) mustReadIO(t *testing.T, offset pi.Address) uint32 {
	t.Helper()
	u32, err := r.pi.ReadWordAt(r.baseAddress + offset)
	if err != nil {
		t.Fatalf("unable to perform PI IO read [offset = %08x]: %v", offset, err)
	}
	return u32
}

func (r *Rom) mustReadBurst(t *testing.T, offset pi.Address, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	if err := r.pi.ReadBurstAt(data, r.baseAddress+offset); err != nil {
		t.Fatalf("unable to perform PI read burst [offset = %08x, size = %08x]: %v", offset, size, err)
	}
	return data
}

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

// ROM should be read in burst that do not cross the 512 byte boundary
// hence the recommended PGS=7 setting.
// Address bit 0 is ignored.
// Length can only be a multiple of 4 (ultrasave limitation ?)
// IORead also ignore the address bit 0
func TestAdmissibleBursts(t *testing.T) {
	r := setup(t)

	t.Logf("0: %08x", r.mustReadIO(t, 0))
	t.Logf("1: %08x", r.mustReadIO(t, 1))
	t.Logf("2: %08x", r.mustReadIO(t, 2))
	t.Logf("3: %08x", r.mustReadIO(t, 3))

	t.Logf("0x20: %08x", r.mustReadIO(t, 0x20))
	t.Logf("0x21: %08x", r.mustReadIO(t, 0x21))
	t.Logf("0x22: %08x", r.mustReadIO(t, 0x22))
	t.Logf("0x23: %08x", r.mustReadIO(t, 0x23))


	t.Log("0, 4", hexDump(r.mustReadBurst(t, 0, 4)))
	t.Log("1, 4", hexDump(r.mustReadBurst(t, 1, 4)))
	t.Log("2, 4", hexDump(r.mustReadBurst(t, 2, 4)))
	t.Log("3, 4", hexDump(r.mustReadBurst(t, 3, 4)))

	t.Log("0, 4", hexDump(r.mustReadBurst(t, 0, 4)))
	t.Log("0, 16", hexDump(r.mustReadBurst(t, 0, 16)))
	t.Log("0, 512", hexDump(r.mustReadBurst(t, 0, 512)))
	// will wrap around after 512 bytes
	t.Log("0, 1024", hexDump(r.mustReadBurst(t, 0, 1024)))

	t.Log("1, 4", hexDump(r.mustReadBurst(t, 1, 4)))
	t.Log("1, 16", hexDump(r.mustReadBurst(t, 1, 16)))
	t.Log("1, 512", hexDump(r.mustReadBurst(t, 1, 512)))
	t.Log("1, 1024", hexDump(r.mustReadBurst(t, 1, 1024)))

	t.Log("2, 4", hexDump(r.mustReadBurst(t, 2, 4)))
	t.Log("2, 16", hexDump(r.mustReadBurst(t, 2, 16)))
	t.Log("2, 512", hexDump(r.mustReadBurst(t, 2, 512)))
	t.Log("2, 1024", hexDump(r.mustReadBurst(t, 2, 1024)))

	t.Log("4, 4", hexDump(r.mustReadBurst(t, 4, 4)))
	t.Log("4, 16", hexDump(r.mustReadBurst(t, 4, 16)))
	t.Log("4, 512", hexDump(r.mustReadBurst(t, 4, 512)))
	t.Log("4, 1024", hexDump(r.mustReadBurst(t, 4, 1024)))

	t.Log("508, 16", hexDump(r.mustReadBurst(t, 508, 16)))
	t.Log("1020, 16", hexDump(r.mustReadBurst(t, 1020, 16)))
}
