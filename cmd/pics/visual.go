package main

import (
	"fmt"
	"io"
)

// Primitives for drawing hexary strings in graphviz dot format

var HexIndexColors = []string{
	"#FFFFFF", // white 0
	"#FBF305", // yellow 1
	"#FF6403", // orange 2
	"#DD0907", // red 3
	"#F20884", // magenta 4
	"#4700A5", // purple 5
	"#0000D3", // blue 6
	"#02ABEA", // cyan 7
	"#1FB714", // green 8
	"#006412", // dark green 9
	"#562C05", // brown A
	"#90713A", // tan B
	"#C0C0C0", // light grey C
	"#808080", // medium grey D
	"#404040", // dark grey E
	"#000000", // black F
}

var HexFontColors = []string{
	"#000000",
	"#000000",
	"#000000",
	"#000000",
	"#000000",
	"#FFFFFF",
	"#FFFFFF",
	"#000000",
	"#000000",
	"#FFFFFF",
	"#FFFFFF",
	"#000000",
	"#000000",
	"#000000",
	"#FFFFFF",
	"#FFFFFF",
}

var hexIndices = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}

func horizontal(w io.Writer, hex []byte, highlighted int, name string, indexColors []string, fontColors []string, compression int) {
	fmt.Fprintf(w,
		`
	%s [label=<
	<table border="0" color="#000000" cellborder="1" cellspacing="0">
	<tr>`, name)
	if hex[len(hex)-1] == 16 {
		hex = hex[:len(hex)-1]
	} else {
		compression = 0 // No compression for non-terminal keys
	}
	for i, h := range hex {
		if i < len(hex)-compression-2 || i > len(hex)-3 {
			if i < highlighted {
				fmt.Fprintf(w,
					`		<td bgcolor="%s"><font color="%s">%s</font></td>
		`, indexColors[h], fontColors[h], hexIndices[h])
			} else {
				fmt.Fprintf(w,
					`		<td bgcolor="%s"></td>
		`, indexColors[h])
			}
		} else if compression > 0 && i == len(hex)-3 {
			fmt.Fprintf(w,
				`		<td border="0">-----------</td>
			`)
		}
	}
	fmt.Fprintf(w,
		`
	</tr></table>
	>];
	`)
}

func startGraph(w io.Writer) {
	fmt.Fprintf(w,
		`digraph trie {
		rankdir=LR;
		node [shape=none margin=0 width=0 height=0]
		edge [dir = none headport=w tailport=e]
	`)
}

func endGraph(w io.Writer) {
	fmt.Fprintf(w,
		`}
`)
}

func circle(w io.Writer, name string, label string, filled bool) {
	if filled {
		fmt.Fprintf(w,
			`%s [label="%s" margin=0.05 shape=Mrecord fillcolor="#E0E0E0" style=filled];
	`, name, label)
	} else {
		fmt.Fprintf(w,
			`%s [label="%s" margin=0.05 shape=Mrecord];
	`, name, label)
	}
}

func Box(w io.Writer, name string, label string) {
	fmt.Fprintf(w,
		`%s [label="%s" shape=box margin=0.1 width=0 height=0 fillcolor="#FF6403" style=filled];
`, name, label)
}

func startCluster(w io.Writer, number int, label string) {
	fmt.Fprintf(w,
		`subgraph cluster_%d {
			label = "%s";
			color = black;
`, number, label)
}

func endCluster(w io.Writer) {
	fmt.Fprintf(w,
		`}
`)
}
