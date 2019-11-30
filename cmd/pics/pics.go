package main

import (
	"flag"
	"fmt"
)

var pic = flag.String("pic", "", "specifies picture to regenerate")

func main() {
	flag.Parse()
	switch *pic {
	case "database_map":
		{
			if err := initialState1(); err != nil {
				fmt.Printf("%v\n", err)
			}
		}
	default:
		{
			fmt.Print("unknown option %s", *pic)
		}
	}
}
