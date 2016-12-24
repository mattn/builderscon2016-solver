package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

var (
	alnum  = runelist{}
	number = runelist{}
)

type node interface {
	expr()
}

type qm struct {
	pat   string
	token []node
	pos   []int
	cand  []string
}

func (x *qm) expr() {}

type marker string

func (x marker) expr() {}

func (x marker) String() string { return string(x) }

type quantum string

func (x quantum) expr() {}

type backref string

func (x backref) expr() {}

type nodelist []node

func (x nodelist) expr() {}

type runelist []rune

func (x runelist) expr() {}

type group struct {
	start int
	end   int
}

func init() {
	for r := 'A'; r <= 'Z'; r++ {
		alnum = append(alnum, r)
	}
	for r := '0'; r <= '9'; r++ {
		alnum = append(alnum, r)
		number = append(number, r)
	}
}

func parseAndWalk(pat string, nest int) ([]node, error) {
	rs := []rune(pat)
	var cc []rune
	var ret []node
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '(':
			i++
			ff := ""
			ret = append(ret, marker(fmt.Sprintf("S%d", nest+1)))
			var cd []node
			for ; i < len(rs) && rs[i] != ')'; i++ {
				if rs[i] == '|' {
					r, err := parseAndWalk(ff, nest+1)
					if err != nil {
						return nil, err
					}
					cd = append(cd, nodelist(r))
					ff = ""
					continue
				}
				ff += string(rs[i])
			}
			r, err := parseAndWalk(ff, nest+1)
			if err != nil {
				return nil, err
			}
			cd = append(cd, nodelist(r))
			ret = append(ret, nodelist(cd))
			ret = append(ret, marker(fmt.Sprintf("E%d", nest+1)))
		case '\\':
			i++
			switch rs[i] {
			case 'd':
				ret = append(ret, number)
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				ret = append(ret, backref("P"+string(rs[i])))
			default:
				return nil, errors.New("invalid token: " + string(rs[i-1:]))
			}
		case '+':
			ret = append(ret, quantum("+"))
		case '[':
			cc = nil
			i++
			if rs[i] == '^' {
				i++
				ff := ""
				for ; i < len(rs) && rs[i] != ']'; i++ {
					ff += string(rs[i])
				}
				for _, a := range alnum {
					if strings.IndexRune(ff, a) == -1 {
						cc = append(cc, a)
					}
				}
			} else {
				for ; i < len(rs) && rs[i] != ']'; i++ {
					cfound := false
					for _, cf := range cc {
						if cf == rs[i] {
							cfound = true
							break
						}
					}
					if !cfound {
						cc = append(cc, rs[i])
					}
				}
			}
			ret = append(ret, runelist(cc))
		case '.':
			ret = append(ret, alnum)
		default:
			ret = append(ret, runelist([]rune{rs[i]}))
		}
	}
	return ret, nil
}

func parse(pat string) ([]node, error) {
	return parseAndWalk(pat, 0)
}

func generate(prefix string, ci int, cs []node, bi int, br [10]group, max int, cb func(s string)) {
	if len(prefix) > max {
		return
	}
	if len(prefix) == max {
		for i := ci; i < len(cs); i++ {
			s, ok := cs[i].(marker)
			if !ok {
				return
			}
			if !strings.HasPrefix(s.String(), "E") {
				return
			}
		}
		cb(prefix)
		return
	}
	for i := ci; i < len(cs); i++ {
		c := cs[i]
		switch t := c.(type) {
		case runelist:
			for _, r := range t {
				generate(prefix+string(r), ci+1, cs, bi, br, max, cb)
			}
			return
		case quantum:
			if ci > 0 {
				for j := 0; j < max-len(prefix); j++ {
					var qp []node
					for jj := 0; jj <= j; jj++ {
						qp = append(qp, cs[i-1])
					}
					qp = append(qp, cs[i+1:]...)
					generate(prefix, 0, qp, bi, br, max, cb)
				}
				return
			}
		case backref:
			if ci > 0 && t[0] == 'P' {
				mp, _ := strconv.Atoi(string(t)[1:])
				part := prefix[br[mp].start:br[mp].end]
				prefix += part
				generate(prefix, ci+1, cs, bi, br, max, cb)
				continue
			}
		case marker:
			if t[0] == 'S' {
				mp, _ := strconv.Atoi(string(t)[1:])
				br[mp].start = len(prefix)
				ci++
				continue
			}
			if ci > 0 && t[0] == 'E' {
				mp, _ := strconv.Atoi(string(t)[1:])
				br[mp].end = len(prefix)
				ci++
				continue
			}
		default:
			for _, ii := range c.(nodelist) {
				iv := ii.(nodelist)
				var cv []node
				for _, iiv := range iv {
					cv = append(cv, iiv)
				}
				cv = append(cv, cs[ci+1:]...)
				generate(prefix, 0, cv, bi, br, max, cb)
			}
		}
	}
}

func main() {
	f, err := os.Open("crossword.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var qs []qm

	// load file to get conditions
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "#") {
			continue
		}
		token := strings.Split(scanner.Text(), "\t")
		var ns []int
		for _, s := range strings.Split(token[1], ",") {
			n, err := strconv.Atoi(s)
			if err != nil {
				log.Fatal(err)
			}
			ns = append(ns, n)
		}

		r, err := parse(token[0])
		if err != nil {
			log.Fatal(err)
		}
		qs = append(qs, qm{token[0], r, ns, nil})
	}

	// generate possible records
	d := make(map[int]map[rune]bool)
	for qi, q := range qs {
		var br [10]group
		generate("", 0, q.token, 0, br, len(q.pos), func(s string) {
			for i, c := range s {
				pos := q.pos[i]

				// ignore KII on center
				if pos == 43 && c != 'K' {
					continue
				}
				if pos == 44 && c != 'I' {
					continue
				}
				if pos == 45 && c != 'I' {
					continue
				}
				if d[pos] == nil {
					d[pos] = make(map[rune]bool)
				}
				d[pos][c] = true
			}
			qs[qi].cand = append(qs[qi].cand, s)
		})
	}

	// find strange record which doesn't cross by anothers.
	for k, _ := range d {
		for _, q := range qs {
			pi := -1
			pq := -1
			for i, pos := range q.pos {
				if pos == k {
					pi = i
					pq = pos
					break
				}
			}
			if pq == -1 {
				continue
			}
			for qi, cand := range q.cand {
				if cand == "" {
					continue
				}
				if _, ok := d[pq][rune(cand[pi])]; !ok {
					q.cand[qi] = ""
				}
			}
		}
	}

	// make ranking
	rank := make(map[int]map[rune]int)
	for _, s := range qs {
		for i, pos := range s.pos {
			for _, cand := range s.cand {
				if cand == "" {
					continue
				}
				if rank[pos] == nil {
					rank[pos] = make(map[rune]int)
				}
				rank[pos][rune(cand[i])]++
			}
		}
	}

	// collect top ranked letters
	gs := make(map[int]string)
	for pos, v := range rank {
		lr := 0
		lc := rune(0)
		for rk, rv := range v {
			if rv > lr {
				lr = rv
				lc = rk
			}
		}

		gs[pos] = string(lc)
	}

	s := gs[12] + gs[13] + " " +
		gs[22] + gs[23] + gs[24] + " " +
		gs[31] + gs[32] + gs[33] + gs[34] + gs[35] + gs[36] + " " +
		gs[43] + gs[44] + gs[45]
	fmt.Println(s)
}
