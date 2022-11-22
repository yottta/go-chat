package tui

import (
	"fmt"
	"github.com/rivo/tview"
	"sync"
)

var NoItemForIdErr = fmt.Errorf("no item found for the given key")

type CList[T any] struct {
	*tview.List

	m                 *sync.Mutex
	items             map[string]clistItem[T]
	itemsIndexes      []string
	itemTextGenerator func(item T) (string, string)
}

func NewCustomList[T any](itemTextGenerator func(item T) (string, string)) *CList[T] {
	return &CList[T]{
		List:              tview.NewList(),
		m:                 &sync.Mutex{},
		items:             map[string]clistItem[T]{},
		itemsIndexes:      []string{},
		itemTextGenerator: itemTextGenerator,
	}
}

func (c *CList[T]) AddItem(id string, item T) {
	c.m.Lock()
	defer c.m.Unlock()
	listItem, ok := c.items[id]
	if ok {
		listItem.obj = item
		c.items[id] = listItem
		mainText, secText := c.itemTextGenerator(item)
		c.List.SetItemText(listItem.idx, listItem.getMainText(mainText), secText)
		return
	}
	c.items[id] = clistItem[T]{
		idx: len(c.itemsIndexes),
		obj: item,
	}
	c.itemsIndexes = append(c.itemsIndexes, id)
	mainText, secText := c.itemTextGenerator(item)
	c.List.AddItem(mainText, secText, 0, nil)
}

func (c *CList[T]) SetUnreadChat(id string, status bool) {
	c.m.Lock()
	defer c.m.Unlock()
	c2, ok := c.items[id]
	if !ok {
		return
	}
	c2.unread = status
	c.items[id] = c2
	mainText, secText := c.itemTextGenerator(c2.obj)
	c.List.SetItemText(c2.idx, c2.getMainText(mainText), secText)
}

func (c *CList[T]) RemoveItem(id string) {
	c.m.Lock()
	defer c.m.Unlock()
	listItem, ok := c.items[id]
	if !ok {
		return
	}
	c.itemsIndexes = append(c.itemsIndexes[:listItem.idx], c.itemsIndexes[listItem.idx+1:]...)
	c.List.RemoveItem(listItem.idx)
	delete(c.items, id)
}

type clistItem[T any] struct {
	idx    int
	obj    T
	unread bool
}

func (ci *clistItem[T]) getMainText(mainText string) string {
	if !ci.unread {
		return mainText
	}
	return "# " + mainText
}
