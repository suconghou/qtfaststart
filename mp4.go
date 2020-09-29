package qtfaststart

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Faststart struct
type Faststart struct {
	r         io.ReadCloser
	size      int64
	atoms     []*Atom
	ftyp      *Atom
	mdat      *Atom
	moov      *Atom
	tasks     chan *loadtasks
	endno     int
	moovtasks chan *loadtasks
	moovEndno int
}

// Atom struct
type Atom struct {
	Type   string // The type of atom
	Offset int64  // The number of bytes between the start of the file and the atom
	Size   int64  // The length of the atom
	Hsize  int8   // The length of the atom's header
}

type convertURL struct {
	f      *Faststart
	url    string
	total  int64
	readed int64
	meta   *struct {
		data *bytes.Buffer
		size int64
	}
	dataMap     map[int]*loadtasks
	played      int
	moovDataMap map[int]*loadtasks
	moovPlayed  int
}

type loadjobs struct {
	playno int
	start  int64
	end    int64
}

type loadtasks struct {
	playno int
	start  int64
	end    int64
	data   *bytes.Buffer
	err    error
}

func (c *convertURL) Read(p []byte) (int, error) {
	if c.meta.data.Len() > 0 {
		n, _ := c.meta.data.Read(p)
		c.readed += int64(n)
		return n, nil
	}
	if c.readed < c.meta.size {
		return 0, fmt.Errorf("meta size not match want %d,got %d", c.meta.size, c.readed)
	}
	if c.readed >= c.meta.size+c.f.moov.Size {
		log.Print("mdat begin")
		os.Exit(0)
		// todo read mdat
		if res, ok := c.dataMap[c.played]; ok {
			if res.data != nil && res.data.Len() > 0 {
				n, err := res.data.Read(p)
				c.readed += int64(n)
				if res.data.Len() == 0 {
					if c.played > 0 && c.played == c.f.endno {
						return n, io.EOF
					}
				}
				if res.err != nil {
					c.played++
				} else {
					// next continue read
				}
				return n, err
			} else if res.err != nil {
				return 0, res.err
			} else {
				c.played++
				return 0, nil
			}
		}
		for {
			log.Print("wait for")
			task := <-c.f.tasks
			c.dataMap[task.playno] = task
			if _, ok := c.dataMap[c.played]; ok {
				return 0, nil
			}
			log.Printf("got part %d , waiting for part %d", task.playno, c.played)
		}
	} else {
		// todo read mdat
		if res, ok := c.moovDataMap[c.moovPlayed]; ok {
			if res.data != nil && res.data.Len() > 0 {
				n, err := res.data.Read(p)
				c.readed += int64(n)
				if res.data.Len() == 0 {
					if c.moovPlayed > 0 && c.moovPlayed == c.f.moovEndno {
						// moov data is end
						return n, nil
					}
				}
				if res.err != nil {
					c.moovPlayed++
				} else {
					// next continue read
				}
				return n, err
			} else if res.err != nil {
				return 0, res.err
			} else {
				c.moovPlayed++
				return 0, nil
			}
		}
		for {
			log.Print("moov wait for")
			task := <-c.f.moovtasks
			c.moovDataMap[task.playno] = task
			if _, ok := c.moovDataMap[c.moovPlayed]; ok {
				return 0, nil
			}
			log.Printf("moov got part %d , waiting for part %d", task.playno, c.moovPlayed)
		}

	}
}

func (c *convertURL) Close() error {
	return nil
}

// New f
func New(r io.ReadCloser, size int64) *Faststart {
	return &Faststart{r: r, size: size}
}

// Parse parse local file
func (f *Faststart) Parse(file *os.File) ([]*Atom, error) {
	var start int64
	for start < f.size {
		if at, err := f.readAtom(start); err == nil {
			switch at.Type {
			case "ftyp":
				f.ftyp = at
			case "mdat":
				f.mdat = at
			case "moov":
				f.moov = at
			case "free", "junk", "pict", "pnot", "skip", "uuid", "wide":
				// Do nothing
			default:
				return nil, fmt.Errorf("%q is not a valid top-level atom", at.Type)
			}
			f.atoms = append(f.atoms, at)
			start, err = file.Seek(start+at.Size, 0)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return f.atoms, nil
}

// ParseURL parse url
func (f *Faststart) ParseURL(url string) ([]*Atom, error) {
	var start int64
	for start < f.size {
		if at, err := f.readAtom(start); err == nil {
			switch at.Type {
			case "ftyp":
				f.ftyp = at
			case "mdat":
				f.mdat = at
			case "moov":
				f.moov = at
			case "free", "junk", "pict", "pnot", "skip", "uuid", "wide":
				// Do nothing
			default:
				return nil, fmt.Errorf("%q is not a valid top-level atom", at.Type)
			}
			f.atoms = append(f.atoms, at)
			start += at.Size
			if at.Size > 8192 {
				reqHeader := http.Header{}
				reqHeader.Set("Range", fmt.Sprintf("bytes=%d-", start))
				resp, err := URLResp(url, reqHeader, 60)
				if err != nil {
					return nil, err
				}
				f.r = resp.Body
			} else {
				_, err = f.readBytes(int(at.Size) - int(at.Hsize))
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, err
		}
	}
	return f.atoms, nil
}

// Convert convert local file
func (f *Faststart) Convert(file *os.File) (io.ReadCloser, error) {
	if len(f.atoms) < 3 {
		_, err := f.Parse(file)
		if err != nil {
			return nil, err
		}
	}
	if f.FastStartEnabled() {
		return file, nil
	}

	return nil, nil
}

// ConvertURL convert by url
func (f *Faststart) ConvertURL(url string) (io.ReadCloser, error) {
	if len(f.atoms) < 3 {
		_, err := f.ParseURL(url)
		if err != nil {
			return nil, err
		}
	}
	if f.FastStartEnabled() {
		resp, err := URLResp(url, nil, 3600)
		if err != nil {
			return nil, err
		}
		return resp.Body, nil
	}
	meta := &struct {
		data *bytes.Buffer
		size int64
	}{data: bytes.NewBuffer(nil)}
	total := f.size
	for _, item := range f.atoms {
		if item.Type == "free" {
			total = total - item.Size
			continue
		}
		if item.Type == "moov" {
			log.Print(item.Offset, item.Size)
			f.moovtasks = make(chan *loadtasks, 8)
			go func(item *Atom) {
				var (
					thread = 4
					chunk  = 10240
					curr   int
					endno  int
					end    = item.Offset + item.Size
					jobs   = make(chan *loadjobs, 8)
				)
				for i := 0; i < thread; i++ {
					go moovworker(f, jobs, url)
				}
				for {
					cstart := int64(curr*chunk) + item.Offset
					cend := cstart + int64(chunk)
					if cend >= end {
						cend = end
						endno = curr
					}
					log.Print("send moov job ", cstart, cend)
					jobs <- &loadjobs{start: cstart, end: cend, playno: curr}
					log.Print("send moov job ok")
					if endno > 0 {
						f.moovEndno = endno
						close(jobs)
						break
					} else {
						curr++
					}
				}
			}(item)
			continue
		}
		if item.Type == "mdat" {
			f.tasks = make(chan *loadtasks, 8)
			go func(item *Atom) {
				var (
					thread = 8
					chunk  = 524288
					curr   int
					endno  int
					end    = item.Offset + item.Size
					jobs   = make(chan *loadjobs, 8)
				)
				for i := 0; i < thread; i++ {
					go worker(f, jobs, url)
				}
				for {
					cstart := int64(curr*chunk) + item.Offset
					cend := cstart + int64(chunk)
					if cend >= end {
						cend = end
						endno = curr
					}
					jobs <- &loadjobs{start: cstart, end: cend, playno: curr}
					if endno > 0 {
						f.endno = endno
						close(jobs)
						break
					} else {
						curr++
					}
				}
			}(item)
			continue
		}
		reqHeader := http.Header{}
		reqHeader.Set("Range", fmt.Sprintf("bytes=%d-%d", item.Offset, item.Offset+item.Size))
		resp, err := URLResp(url, reqHeader, 600)
		if err != nil {
			return nil, err
		}
		f.r = resp.Body
		bs, err := f.readBytes(int(item.Size))
		defer resp.Body.Close()
		if err != nil {
			return nil, err
		}
		_, err = meta.data.Write(bs)
		if err != nil {
			return nil, err
		}
		meta.size += item.Size
	}
	return &convertURL{f: f, url: url, meta: meta, total: total, dataMap: make(map[int]*loadtasks, 8), moovDataMap: make(map[int]*loadtasks, 8)}, nil
}

// FastStartEnabled return bool
func (f *Faststart) FastStartEnabled() bool {
	return f.moov.Offset < f.mdat.Offset
}

func (f *Faststart) readAtom(offset int64) (*Atom, error) {
	at := &Atom{Offset: offset, Hsize: 8}
	bs, err := f.readBytes(8)
	if err != nil {
		return nil, err
	}
	// The first 4 bytes contain the size of the atom
	at.Size = int64(binary.BigEndian.Uint32(bs[0:4]))
	// The next 4 bytes contain the type of the atom
	at.Type = string(bs[4:])
	// If the size is 1, look at the next 8 bytes for the real size
	if at.Size == 1 {
		bs, err = f.readBytes(8)
		if err != nil {
			return nil, err
		}
		at.Size = int64(binary.BigEndian.Uint64(bs))
		at.Hsize += 8
	}
	return at, nil
}

func (f *Faststart) readBytes(chunk int) ([]byte, error) {
	buf := make([]byte, chunk)
	n, err := io.ReadFull(f.r, buf)
	if err != nil {
		return nil, err
	}
	if n != chunk {
		return nil, fmt.Errorf("read not full want %d,got %d", chunk, n)
	}
	return buf, nil
}

// URLResp return http response
func URLResp(url string, reqHeader http.Header, timeout int) (*http.Response, error) {
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if reqHeader == nil {
		reqHeader = http.Header{}
		reqHeader.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Safari/537.36")
	}
	req.Header = reqHeader
	return client.Do(req)
}

func worker(f *Faststart, jobs chan *loadjobs, url string) {
	for {
		job, more := <-jobs
		if more {
			resp, err := loadItem(job.start, job.end, url)
			if err != nil {
				f.tasks <- &loadtasks{data: nil, playno: job.playno, err: err}
			} else {
				data, err := getItem(resp, job.start, job.end, job.playno, url)
				f.tasks <- &loadtasks{data: data, playno: job.playno, err: err}
			}
		} else {
			return
		}
	}
}

func moovworker(f *Faststart, jobs chan *loadjobs, url string) {
	for {
		job, more := <-jobs
		if more {
			log.Print("got job ", job.start, job.end)
			resp, err := loadItem(job.start, job.end, url)
			if err != nil {
				f.moovtasks <- &loadtasks{data: nil, playno: job.playno, err: err}
			} else {
				data, err := getItem(resp, job.start, job.end, job.playno, url)
				f.moovtasks <- &loadtasks{data: data, playno: job.playno, err: err}
			}
		} else {
			return
		}
	}
}

func loadItem(start int64, end int64, url string) (*http.Response, error) {
	var (
		reqHeader = http.Header{}
		timeout   = int(end-start) / 1024 / 4
	)
	reqHeader.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
	return URLResp(url, reqHeader, timeout)
}

func getItem(resp *http.Response, start int64, end int64, playno int, url string) (*bytes.Buffer, error) {
	var (
		data     = bytes.NewBuffer(nil)
		cstart   int64
		trytimes uint8
		maxtimes uint8 = 5
		buf            = make([]byte, 262144)
	)
	defer resp.Body.Close()
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			data.Write(buf[0:n])
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return data, nil
		}
		trytimes++
		if trytimes > maxtimes {
			return data, err
		}
		time.Sleep(time.Second)
		cstart = start + int64(data.Len())
		resp, err = loadItem(cstart, end, url)
		if err != nil {
			return data, err
		}
	}
}
