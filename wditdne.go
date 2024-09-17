package wditdne

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"slices"
	"sort"
	"strings"
)

type Jpeg struct {
	infile *os.File
	hiding
	headers            []jpegHeader
	quantizationTables []quantizationTable
	huffmanTables      []huffmanTable
	frame              frame
	sospos             int
	tmp                *os.File
}

type jpegHeader struct {
	marker    [2]byte
	markerStr string
	len       int
	data      []byte
	qtIdx     []int
	htIdx     []int
}

type frame struct {
	p          int
	y          int
	x          int
	nf         int
	components []component
	ymcus      int
	xmcus      int
}

type component struct {
	id        int
	h         int
	v         int
	tq        int
	qtIdx     int
	htdcIdx   int
	htacIdx   int
	htacIdxEx int
	prevdc    int
}

type quantizationTable struct {
	pq    int
	tq    int
	table [64]byte
}

type huffmanTable struct {
	tc    int
	th    int
	l     [16]byte
	v     [16][]byte
	table map[string]byte
}

func (ht *huffmanTable) buildTable() {
	ht.table = make(map[string]byte)
	code := 0
	for i, htv := range ht.v {
		ht.l[i] = byte(len(htv))
		for _, v := range htv {
			bin := fmt.Sprintf("%b", code)
			if len(bin) < i+1 {
				pad := "%0" + fmt.Sprintf("%d", i+1) + "s"
				bin = fmt.Sprintf(pad, bin)
			}
			ht.table[bin] = v
			code++
		}
		code <<= 1
	}
}

var ghtacl = [16][]byte{
	{},
	{0x01, 0x02},
	{0x03},
	{0x00, 0x04, 0x11},
	{0x05, 0x12, 0x21},
	{0x31, 0x41},
	{0x06, 0x13, 0x51, 0x61},
	{0x07, 0x22, 0x71},
	{0x14, 0x32, 0x81, 0x91, 0xa1},
	{0x08, 0x23, 0x42, 0xb1, 0xc1},
	{0x15, 0x52, 0xd1, 0xf0},
	{0x24, 0x33, 0x62, 0x72},
	{},
	{},
	{0x82},
	{
		0x09, 0x0a, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x25,
		0x26, 0x27, 0x28, 0x29, 0x2a, 0x34, 0x35, 0x36,
		0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45, 0x46,
		0x47, 0x48, 0x49, 0x4a, 0x53, 0x54, 0x55, 0x56,
		0x57, 0x58, 0x59, 0x5a, 0x63, 0x64, 0x65, 0x66,
		0x67, 0x68, 0x69, 0x6a, 0x73, 0x74, 0x75, 0x76,
		0x77, 0x78, 0x79, 0x7a, 0x83, 0x84, 0x85, 0x86,
		0x87, 0x88, 0x89, 0x8a, 0x92, 0x93, 0x94, 0x95,
		0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4,
		0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3,
		0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2,
		0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca,
		0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9,
		0xda, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7,
		0xe8, 0xe9, 0xea, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5,
		0xf6, 0xf7, 0xf8, 0xf9, 0xfa,
	},
}

var ghtacc = [16][]byte{
	{},
	{0x00, 0x01},
	{0x02},
	{0x03, 0x11},
	{0x04, 0x05, 0x21, 0x31},
	{0x06, 0x12, 0x41, 0x51},
	{0x07, 0x61, 0x71},
	{0x13, 0x22, 0x32, 0x81},
	{0x08, 0x14, 0x42, 0x91, 0xa1, 0xb1, 0xc1},
	{0x09, 0x23, 0x33, 0x52, 0xf0},
	{0x15, 0x62, 0x72, 0xd1},
	{0x0a, 0x16, 0x24, 0x34},
	{},
	{0xe1},
	{0x25, 0xf1},
	{
		0x17, 0x18, 0x19, 0x1a, 0x26, 0x27, 0x28, 0x29,
		0x2a, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x43,
		0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x53,
		0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x63,
		0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x73,
		0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7a, 0x82,
		0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a,
		0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99,
		0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8,
		0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7,
		0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6,
		0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4, 0xd5,
		0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe2, 0xe3, 0xe4,
		0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xf2, 0xf3,
		0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa,
	},
}

type hiding struct {
	data   io.Reader
	depth  int
	repeat bool
}

func NewJpeg(infile *os.File) (*Jpeg, error) {
	j := &Jpeg{
		infile: infile,
	}
	err := j.prepare()
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (j *Jpeg) Hide(data io.Reader, depth int, repeat bool, outfile io.Writer) error {
	f, err := os.CreateTemp("", "wditdne")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	j.tmp = f
	bytes2 := make([]byte, 2)
	j.hiding = hiding{
		data:   data,
		depth:  depth,
		repeat: repeat,
	}
	j.infile.Seek(int64(j.sospos), io.SeekStart)
	br := newBitReader(j.infile)
	bw := newBitWriter(j.tmp)
	// ready for encoding with general ac huffman tables
	for ic := range j.frame.components {
		comp := &j.frame.components[ic]
		htacIdx := comp.htacIdx
		ohtac := j.huffmanTables[htacIdx]
		ght := ghtacc
		if comp.id == 1 {
			ght = ghtacl
		}
		ehtac := huffmanTable{
			tc: ohtac.tc,
			th: ohtac.th,
			v:  ght,
		}
		ehtac.buildTable()
		j.huffmanTables = append(j.huffmanTables, ehtac)
		comp.htacIdxEx = len(j.huffmanTables) - 1

		qt := &j.quantizationTables[comp.qtIdx].table
		qts := *qt
		if !slices.Contains(qts[:], 0) {
			sort.Slice(qts[:], func(i, j int) bool {
				return qts[i] > qts[j]
			})
			for i := 0; i < j.hiding.depth; i++ {
				n := qts[i]
				for j := len(qt) - 1; j >= 0; j-- {
					if qt[j] == n {
						qt[j] = 0
						break
					}
				}
			}
		}
	}

	for uy := 0; uy < j.frame.ymcus; uy++ {
		for ux := 0; ux < j.frame.xmcus; ux++ {
			for ic := range j.frame.components {
				comp := j.frame.components[ic]
				for v := 0; v < comp.v; v++ {
					for h := 0; h < comp.h; h++ {
						// read qCoeffs
						qCoeffs := [64]int{}
						// dc
						htdc := &j.huffmanTables[comp.htdcIdx]
						_, t := j.decodeHuffman(br, htdc)
						diff := 0
						if t != 0 {
							diff, _ = br.readNBits(t)
						}
						prevdc := comp.prevdc
						comp.prevdc += diff
						qCoeffs[0] = comp.prevdc

						// ac
						htac := &j.huffmanTables[comp.htacIdx]
						k := 1
						for k < 64 {
							_, rs := j.decodeHuffman(br, htac)
							r := rs >> 4
							s := rs & 0x0f
							if s == 0 {
								if r == 15 {
									k += 16
									continue
								} else {
									break
								}
							}
							k += r
							ac, _ := br.readNBits(s)
							qCoeffs[k] = ac
							k++
						}

						// ----------------------
						// modify to hide data
						qt := j.quantizationTables[comp.qtIdx].table
						for i, v := range qt {
							if v == 0 {
								b := make([]byte, 1)
								n, _ := j.hiding.data.Read(b)
								if j.hiding.repeat && n == 0 {
									seeker, ok := j.hiding.data.(io.Seeker)
									if ok {
										seeker.Seek(0, io.SeekStart)
										j.hiding.data.Read(b)
									}
								}
								qCoeffs[i] = int(b[0])
							}
						}

						// ----------------------
						// rebuild qCoeffs
						htac = &j.huffmanTables[comp.htacIdxEx]
						// dc
						xdiff := qCoeffs[0] - prevdc
						adiff := xdiff
						if adiff < 0 {
							xdiff = xdiff - 1
							adiff = -adiff
						}
						length := bits.Len(uint(adiff))
						j.encodeHuffman(bw, htdc, length)
						bw.writeNBits(xdiff&(1<<length-1), length)

						// ac
						rle := 0
						for k = 1; k < 64; k++ {
							ac := qCoeffs[k]
							if ac == 0 {
								rle++
							} else {
								aac := ac
								if ac < 0 {
									ac = ac - 1
									aac = -aac
								}
								for rle > 15 {
									j.encodeHuffman(bw, htac, 0xf0)
									rle -= 16
								}
								length = bits.Len(uint(aac))
								rs := rle<<4 | length
								j.encodeHuffman(bw, htac, rs)
								bw.writeNBits(ac&(1<<length-1), length)
								rle = 0
							}
						}
						if rle > 0 {
							j.encodeHuffman(bw, htac, 0x00)
						}
					}
				}

			}
		}
	}
	bw.flush()

	// To output
	outfile.Write([]byte{0xff, 0xd8})
	for _, header := range j.headers {
		switch header.markerStr {
		case "ffdb": // dqt
			for _, i := range header.qtIdx {
				qt := j.quantizationTables[i]
				header.data = []byte{}
				header.data = append(header.data, byte(qt.pq<<4|qt.tq))
				header.data = append(header.data, qt.table[:]...)
			}
		case "ffc4": // dht
			for _, idx := range header.htIdx {
				iex := idx
				for ic := range j.frame.components {
					comp := j.frame.components[ic]
					if comp.htacIdx == idx {
						iex = comp.htacIdxEx
						break
					}
				}
				ht := j.huffmanTables[iex]
				header.data = []byte{}
				header.data = append(header.data, byte(ht.tc<<4|ht.th))
				for i := 0; i < 16; i++ {
					header.data = append(header.data, byte(len(ht.v[i])))
				}
				for i := 0; i < 16; i++ {
					header.data = append(header.data, ht.v[i]...)
				}

			}
			header.len = len(header.data) + 2
		}

		outfile.Write(header.marker[:])
		binary.BigEndian.PutUint16(bytes2, uint16(header.len))
		outfile.Write(bytes2)
		outfile.Write(header.data)
	}
	j.tmp.Seek(0, io.SeekStart)
	bufio.NewReader(j.tmp).WriteTo(outfile)
	outfile.Write([]byte{0xff, 0xd9})

	return nil
}

func (j *Jpeg) Extract(verbose bool) (string, error) {
	c := 0
	hidden := []byte{}
	details := []string{"QUANTIZED COEFICIENTS AND HIDDEN DATA:\n"}
	j.infile.Seek(int64(j.sospos), io.SeekStart)
	br := newBitReader(j.infile)
	for uy := 0; uy < j.frame.ymcus; uy++ {
		for ux := 0; ux < j.frame.xmcus; ux++ {
			for ic := range j.frame.components {
				comp := j.frame.components[ic]
				for v := 0; v < comp.v; v++ {
					for h := 0; h < comp.h; h++ {
						c++
						// read qCoeffs
						qCoeffs := [64]int{}
						// dc
						htdc := &j.huffmanTables[comp.htdcIdx]
						_, t := j.decodeHuffman(br, htdc)
						diff := 0
						if t != 0 {
							diff, _ = br.readNBits(t)
						}
						comp.prevdc += diff
						qCoeffs[0] = comp.prevdc

						// ac
						htac := &j.huffmanTables[comp.htacIdx]
						k := 1
						for k < 64 {
							_, rs := j.decodeHuffman(br, htac)
							r := rs >> 4
							s := rs & 0x0f
							if s == 0 {
								if r == 15 {
									k += 16
									continue
								} else {
									break
								}
							}
							k += r
							ac, _ := br.readNBits(s)
							qCoeffs[k] = ac
							k++
						}

						qt := j.quantizationTables[comp.qtIdx].table
						if verbose {
							coe := []string{}
							rev := []string{}
							for i, v := range qCoeffs {
								if i == 0 {
									coe = append(coe, "["+fmt.Sprintf("%4d", v))
									rev = append(rev, "     ")
								} else {
									coe = append(coe, fmt.Sprintf("%3d", v))
									if qt[i] == 0 {
										rev = append(rev, "  "+string(v))
									} else {
										rev = append(rev, "   ")
									}
								}
							}
							coes := strings.Join(coe, " ") + "]"
							revs := strings.Join(rev, " ") + " "
							details = append(details, coes+"\n"+revs+"\n")
						}
						for i, v := range qt {
							if v == 0 {
								hidden = append(hidden, byte(qCoeffs[i]))
							}
						}
					}
				}

			}
		}
	}
	data := ""
	if !verbose {
		data = string(hidden) + "\n"
	} else {
		nblocks := 0
		details = append(details, "\nQUANTIZATION TABLES:\n")
		for _, c := range j.frame.components {
			nblocks += c.h * c.v
			q := j.quantizationTables[c.qtIdx]
			details = append(details, fmt.Sprintf("%d\n", q.table))
		}
		data = strings.Join(details, "")
		nblocks *= j.frame.ymcus * j.frame.xmcus
		data += fmt.Sprintf("\nNUMBER OF BLOCKS:\n%d\n", nblocks)
	}

	return data, nil
}

func (j *Jpeg) prepare() error {
	byte1 := make([]byte, 1)
	bytes2 := make([]byte, 2)
	j.infile.Read(bytes2) // ffd8
	for {
		j.infile.Read(bytes2)
		markerStr := bytes2maker(bytes2)
		marker := [2]byte{bytes2[0], bytes2[1]}
		j.infile.Read(bytes2)
		len := bytes2length(bytes2)
		bytes := make([]byte, len-2)
		j.infile.Read(bytes)
		j.headers = append(j.headers, jpegHeader{
			marker:    marker,
			markerStr: markerStr,
			len:       len,
			data:      bytes,
		})

		if markerStr == "ffda" {
			pos, _ := j.infile.Seek(0, io.SeekCurrent)
			j.sospos = int(pos)
			break
		}
	}

	for i := range j.headers {
		header := &j.headers[i]
		switch header.markerStr {
		case "ffda": // sos
			br := bytes.NewReader(header.data)
			br.Read(byte1)
			n := int(byte1[0])
			for i := 0; i < n; i++ {
				br.Read(byte1)
				id := int(byte1[0])
				br.Read(byte1)
				td, ta := read4bits(byte1[0])
				for i := range j.frame.components {
					comp := &j.frame.components[i]
					if comp.id == id {
						for ih, ht := range j.huffmanTables {
							if ht.tc == 0 && ht.th == td {
								comp.htdcIdx = ih
							} else if ht.tc == 1 && ht.th == ta {
								comp.htacIdx = ih
							}
						}
					}
				}
			}

		case "ffc0": // sof0
			br := bytes.NewReader(header.data)
			br.Read(byte1)
			j.frame.p = int(byte1[0])
			br.Read(bytes2)
			j.frame.y = int(bytes2length(bytes2))
			br.Read(bytes2)
			j.frame.x = int(bytes2length(bytes2))
			br.Read(byte1)
			j.frame.nf = int(byte1[0])
			maxv := 0
			maxh := 0
			for i := 0; i < j.frame.nf; i++ {
				component := component{}
				br.Read(byte1)
				component.id = int(byte1[0])
				br.Read(byte1)
				component.h, component.v = read4bits(byte1[0])
				if component.h > maxh {
					maxh = component.h
				}
				if component.v > maxv {
					maxv = component.v
				}
				br.Read(byte1)
				component.tq = int(byte1[0])
				for iq, qt := range j.quantizationTables {
					if qt.tq == component.tq {
						component.qtIdx = iq
					}
				}
				j.frame.components = append(j.frame.components, component)
			}
			j.frame.ymcus = int(math.Ceil(float64(j.frame.y) / 8.0 / float64(maxv)))
			j.frame.xmcus = int(math.Ceil(float64(j.frame.x) / 8.0 / float64(maxh)))

		case "ffdb": // dqt
			br := bytes.NewReader(header.data)
			bytes64 := make([]byte, 64)
			for {
				qt := quantizationTable{}
				br.Read(byte1)
				qt.pq, qt.tq = read4bits(byte1[0])
				br.Read(bytes64)
				copy(qt.table[:], bytes64)
				j.quantizationTables = append(j.quantizationTables, qt)
				header.qtIdx = append(header.qtIdx, len(j.quantizationTables)-1)
				pos, _ := br.Seek(0, io.SeekCurrent)
				if pos >= int64(len(header.data)) {
					break
				}
			}
		case "ffc4": // dht
			br := bytes.NewReader(header.data)
			bytes16 := make([]byte, 16)
			for {
				ht := huffmanTable{}
				br.Read(byte1)
				ht.tc, ht.th = read4bits(byte1[0])
				br.Read(bytes16)
				copy(ht.l[:], bytes16)
				for i := 0; i < 16; i++ {
					v := make([]byte, ht.l[i])
					br.Read(v)
					ht.v[i] = v
				}
				ht.buildTable()
				j.huffmanTables = append(j.huffmanTables, ht)
				header.htIdx = append(header.htIdx, len(j.huffmanTables)-1)
				pos, _ := br.Seek(0, io.SeekCurrent)
				if pos >= int64(len(header.data)) {
					break
				}
			}

		}
	}

	return nil
}

func (j *Jpeg) decodeHuffman(br *bitReader, ht *huffmanTable) (string, int) {
	code := ""
	for i := 0; i < 16; i++ {
		b, _ := br.readBit()
		code += fmt.Sprintf("%d", b)
		v, ok := ht.table[code]
		if ok {
			return code, int(v)
		}
	}
	return "", 0
}

func (j *Jpeg) encodeHuffman(br *bitWriter, ht *huffmanTable, value int) string {
	for k, v := range ht.table {
		if v == byte(value) {
			c := []rune(k)
			for i := 0; i < len(c); i++ {
				br.writeBit(int(c[i] - '0'))
			}
			return k
		}
	}
	return ""
}

func bytes2maker(bytes2 []byte) string {
	return hex.EncodeToString(bytes2)
}

func bytes2length(bytes2 []byte) int {
	return int(binary.BigEndian.Uint16(bytes2))
}

func read4bits(b byte) (int, int) {
	return int(b >> 4), int(b & 0x0f)
}

type bitReader struct {
	r   io.Reader
	buf []byte
	pos int
}

func newBitReader(r io.Reader) *bitReader {
	b := make([]byte, 1)
	return &bitReader{
		r:   r,
		buf: b,
	}
}

func (br *bitReader) readBit() (int, error) {
	if br.pos == 0 {
		_, err := br.r.Read(br.buf)
		if err != nil {
			return 0, err
		}
		if br.buf[0] == 0xff {
			b0 := make([]byte, 1)
			br.r.Read(b0)
		}
	}
	bit := int(br.buf[0] >> (7 - br.pos) & 0x01)
	br.pos = (br.pos + 1) % 8
	return bit, nil
}

func (br *bitReader) readNBits(length int) (int, error) {
	n := 0
	l := length
	for l > 0 {
		b, err := br.readBit()
		if err != nil {
			return 0, err
		}
		n = n<<1 | b
		l--
	}
	if n < 1<<(length-1) {
		n = n + (-1 << length) + 1
	}
	return n, nil
}

type bitWriter struct {
	w   io.Writer
	buf byte
	pos int
}

func newBitWriter(w io.Writer) *bitWriter {
	return &bitWriter{
		w: w,
	}
}

func (bw *bitWriter) writeBit(bit int) error {
	bw.buf = bw.buf | byte(bit<<(7-bw.pos))
	bw.pos++
	if bw.pos == 8 {
		_, err := bw.w.Write([]byte{bw.buf})
		if bw.buf == 0xff {
			_, err = bw.w.Write([]byte{0x00})
		}
		if err != nil {
			return err
		}
		bw.buf = 0
		bw.pos = 0
	}
	return nil
}

func (bw *bitWriter) writeNBits(n int, length int) error {
	for i := length - 1; i >= 0; i-- {
		err := bw.writeBit(int(n>>i) & 0x01)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bw *bitWriter) flush() error {
	if bw.pos > 0 {
		_, err := bw.w.Write([]byte{bw.buf})
		if err != nil {
			return err
		}
	}
	return nil
}
