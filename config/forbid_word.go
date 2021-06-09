package config

type trieNode struct {
	tail   bool
	value  rune
	childs map[rune]*trieNode
}

// 字典数
type trie struct {
	head *trieNode
}

func newTrie(words []string) *trie {
	t := &trie{
		head: &trieNode{
			value:  rune('R'),
			childs: make(map[rune]*trieNode),
			tail:   false,
		},
	}
	for _, w := range words {
		p := t.head
		for k, r := range []rune(w) {
			q, ok := p.childs[r]
			if !ok {
				q = &trieNode{value: r, childs: make(map[rune]*trieNode)}
				p.childs[r] = q
			}
			p = q
			if k+1 == len([]rune(w)) {
				p.tail = true
			}
		}
	}
	return t
}

var forbidWordsTrie *trie

func ForbidWords(msg string) string {
	if forbidWordsTrie == nil {
		return msg
	}
	msg += "'"

	buf := []rune(msg)
	size := len(buf)
	res := make([]rune, 0, 16)
	for i := 0; i < size; {
		end := -1
		p := forbidWordsTrie.head
		for k := i; k < size; k++ {
			w := buf[k]
			q, ok := p.childs[w]
			if !ok {
				break
			}
			if q.tail {
				end = k
			}
			p = q
		}
		if end == -1 {
			res = append(res, buf[i])
			i = i + 1
		} else {
			for x := i; x <= end; x++ {
				res = append(res, rune('*'))
			}
			i = end + 1
		}
	}
	s := string(res)
	return s[:len(s)-1]
}

func LoadForbidWords(words []string) {
	forbidWordsTrie = newTrie(words)
}
