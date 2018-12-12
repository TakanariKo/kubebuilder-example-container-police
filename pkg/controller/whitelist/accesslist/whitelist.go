package accesslist

import (
	"sync"
)

// WhiteList struct is to manage whitelist
type WhiteList struct {
	mu        sync.RWMutex
	whitelist map[string][]string
}

// NewWhiteList is to create new instance
func NewWhiteList() *WhiteList {
	return &WhiteList{
		whitelist: make(map[string][]string),
	}
}

// Add image name and tag info to WhiteList
func (l *WhiteList) Add(image string, tag string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.whitelist[image] = append(l.whitelist[image], tag)
}

// Search image name and tag. Not support content digest
func (l *WhiteList) Search(image string, tag string) bool {
	if _, ok := l.whitelist[image]; !ok {
		return false
	}
	for _, v := range l.whitelist[image] {
		if tag == v {
			return true
		}
	}
	return false
}
