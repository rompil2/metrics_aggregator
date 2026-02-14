package example

import (
	"fmt"

	"github.com/rompil2/metrics_aggregator/internal/pool"
)

// generate:reset
type ResetableStruct struct {
	i     int
	str   string
	strP  *string
	s     []int
	m     map[string]string
	child *ResetableStruct
}

func ExamplePool() {
	p := pool.New(func() *ResetableStruct {
		return &ResetableStruct{}
	})

	// Get a value from the pool
	rs := p.Get()
	// Modify the value like it is a real usage
	rs.i = 1
	rs.m = map[string]string{}
	rs.str = "test"
	rs.strP = &rs.str
	rs.s = []int{1, 2, 3}
	if rs.child == nil {
		rs.child = &ResetableStruct{
			i: 1,
		}
	}
	fmt.Printf("Got: %+v\n", rs)
	p.Put(rs)
	rs2 := p.Get()
	fmt.Printf("Got again: %+v\n", rs2)
}
