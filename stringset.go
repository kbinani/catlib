package catlib

import (
	"sort"
)

type StringSet struct {
	values map[string]struct{}
}

func NewStringSet() *StringSet {
	o := new(StringSet)
	o.values = make(map[string]struct{})
	return o
}

func (o *StringSet) Put(s string) {
	o.values[s] = struct{}{}
}

func (this *StringSet) Has(s string) bool {
	_, ok := this.values[s]
	return ok
}

func (o *StringSet) Values() []string {
	ret := []string{}
	for v := range o.values {
		ret = append(ret, v)
	}
	return ret
}

func (o *StringSet) SortedValues() []string {
	ret := o.Values()
	sort.Strings(ret)
	return ret
}

func (o *StringSet) Merge(other *StringSet) {
	for v := range other.values {
		o.values[v] = struct{}{}
	}
}

func (o *StringSet) Size() int {
	return len(o.values)
}

func (this *StringSet) Del(s string) {
	delete(this.values, s)
}
