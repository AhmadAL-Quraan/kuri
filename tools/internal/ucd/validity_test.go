// Copyright (c) 2026 dexpace and Omar Aljarrah
// SPDX-License-Identifier: MIT

package ucd

import "testing"

func TestLoadValidityUnicodeDataMarkAndVirama(t *testing.T) {
	// 0300 is COMBINING GRAVE ACCENT: General_Category Mn, CCC 230 (not Virama).
	// 094D is DEVANAGARI SIGN VIRAMA: General_Category Mn, CCC 9 (Virama) — so it
	// must show up in BOTH the marks and the viramas sets.
	// 0041 is LATIN CAPITAL LETTER A: General_Category Lu, CCC 0 — neither.
	data := []byte(
		"0041;LATIN CAPITAL LETTER A;Lu;0;L;;;;;N;;;;;\n" +
			"0300;COMBINING GRAVE ACCENT;Mn;230;NSM;;;;;N;;;;;\n" +
			"094D;DEVANAGARI SIGN VIRAMA;Mn;9;NSM;;;;;N;;;;;\n",
	)
	marks, viramas, err := loadValidityUnicodeData(data)
	if err != nil {
		t.Fatalf("loadValidityUnicodeData: unexpected error: %v", err)
	}
	if len(marks) != 2 || marks[0].start != 0x300 || marks[1].start != 0x94D {
		t.Fatalf("marks = %+v, want ranges at 0x300 and 0x94D", marks)
	}
	if len(viramas) != 1 || viramas[0].start != 0x94D {
		t.Fatalf("viramas = %+v, want a single range at 0x94D", viramas)
	}
}

func TestLoadValidityUnicodeDataFirstLastBlock(t *testing.T) {
	// A <..., First>/<..., Last> block must expand to cover every code point in
	// the enclosed run, taking its category/CCC from the Last (closing) row.
	data := []byte(
		"3400;<CJK Ideograph Extension A, First>;Lo;0;L;;;;;N;;;;;\n" +
			"4DBF;<CJK Ideograph Extension A, Last>;Lo;0;L;;;;;N;;;;;\n",
	)
	marks, viramas, err := loadValidityUnicodeData(data)
	if err != nil {
		t.Fatalf("loadValidityUnicodeData: unexpected error: %v", err)
	}
	// Lo is not a Mark category and CCC 0 is not Virama, so this block should
	// contribute nothing to either set — but it must not error out.
	if len(marks) != 0 || len(viramas) != 0 {
		t.Fatalf("marks=%+v viramas=%+v, want both empty for a non-mark/non-virama block", marks, viramas)
	}
}

func TestLoadValidityUnicodeDataFirstLastBlockAsMark(t *testing.T) {
	// Same shape as above, but the Last row is Mn/CCC 230, so the ENTIRE expanded
	// range must land in marks with the correct bounds (start from First, end
	// from Last).
	data := []byte(
		"1AB0;<hypothetical mark block, First>;Mn;230;NSM;;;;;N;;;;;\n" +
			"1AB4;<hypothetical mark block, Last>;Mn;230;NSM;;;;;N;;;;;\n",
	)
	marks, _, err := loadValidityUnicodeData(data)
	if err != nil {
		t.Fatalf("loadValidityUnicodeData: unexpected error: %v", err)
	}
	if len(marks) != 1 || marks[0].start != 0x1AB0 || marks[0].end != 0x1AB4 {
		t.Fatalf("marks = %+v, want a single range [0x1AB0, 0x1AB4]", marks)
	}
}

func TestLoadValidityUnicodeDataInvalidCodePoint(t *testing.T) {
	data := []byte("ZZZZ;BOGUS;Lu;0;L;;;;;N;;;;;\n")
	if _, _, err := loadValidityUnicodeData(data); err == nil {
		t.Fatal("loadValidityUnicodeData with invalid code point: expected error, got nil")
	}
}

func TestLoadValidityUnicodeDataInvalidCCC(t *testing.T) {
	data := []byte("0041;LATIN CAPITAL LETTER A;Lu;NOTANUMBER;L;;;;;N;;;;;\n")
	if _, _, err := loadValidityUnicodeData(data); err == nil {
		t.Fatal("loadValidityUnicodeData with invalid CCC: expected error, got nil")
	}
}

func TestLoadJoiningBasic(t *testing.T) {
	data := []byte(
		"0620          ; D # ARABIC LETTER KASHMIRI YEH\n" +
			"0621          ; U # ARABIC LETTER HAMZA (Non_Joining, dropped)\n" +
			"0622..0623    ; R # small range, Right_Joining\n" +
			"\n" +
			"# a comment-only line\n",
	)
	got, err := loadJoining(data)
	if err != nil {
		t.Fatalf("loadJoining: unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("loadJoining: got %d ranges, want 2 (U type must be dropped)", len(got))
	}
	if got[0].start != 0x620 || got[0].end != 0x620 || got[0].jtype != "D" {
		t.Errorf("got[0] = %+v, want start=0x620 end=0x620 type=D", got[0])
	}
	if got[1].start != 0x622 || got[1].end != 0x623 || got[1].jtype != "R" {
		t.Errorf("got[1] = %+v, want start=0x622 end=0x623 type=R", got[1])
	}
}

func TestLoadJoiningMalformedRecord(t *testing.T) {
	// More than one ';' in a data line is malformed.
	data := []byte("0620; D; extra\n")
	if _, err := loadJoining(data); err == nil {
		t.Fatal("loadJoining with a malformed record: expected error, got nil")
	}
}

func TestLoadJoiningInvalidCodeRange(t *testing.T) {
	data := []byte("ZZZZ; D\n")
	if _, err := loadJoining(data); err == nil {
		t.Fatal("loadJoining with an invalid code range: expected error, got nil")
	}
}

func TestMergeSetRangesMergesTouchingAndOverlapping(t *testing.T) {
	in := []plainRange{
		{start: 10, end: 20},
		{start: 21, end: 25}, // touches (21 == 20+1)
		{start: 24, end: 30}, // overlaps the merged [10,25]
		{start: 100, end: 110},
	}
	got := mergeSetRanges(in)
	if len(got) != 2 {
		t.Fatalf("mergeSetRanges: got %d ranges, want 2: %+v", len(got), got)
	}
	if got[0].start != 10 || got[0].end != 30 {
		t.Errorf("got[0] = %+v, want [10, 30]", got[0])
	}
	if got[1].start != 100 || got[1].end != 110 {
		t.Errorf("got[1] = %+v, want [100, 110]", got[1])
	}
}

func TestMergeSetRangesSeparatedByOneMerges(t *testing.T) {
	// mergeSetRanges' adjacency test is current.start <= merged.end+1, so a range
	// separated from the previous by exactly one code point (a single-point gap)
	// should still merge.
	in := []plainRange{
		{start: 0, end: 5},
		{start: 6, end: 10},
	}
	got := mergeSetRanges(in)
	if len(got) != 1 || got[0].start != 0 || got[0].end != 10 {
		t.Fatalf("mergeSetRanges = %+v, want a single merged [0, 10]", got)
	}
}

func TestMergeSetRangesNoMergeWithGap(t *testing.T) {
	in := []plainRange{
		{start: 0, end: 5},
		{start: 7, end: 10}, // gap at 6
	}
	got := mergeSetRanges(in)
	if len(got) != 2 {
		t.Fatalf("mergeSetRanges = %+v, want 2 separate ranges (gap at 6)", got)
	}
}

func TestMergeTypedRangesMergesSameType(t *testing.T) {
	in := []typedRange{
		{start: 0, end: 5, jtype: "L"},
		{start: 6, end: 10, jtype: "L"},
	}
	got := mergeTypedRanges(in)
	if len(got) != 1 || got[0].end != 10 {
		t.Fatalf("mergeTypedRanges = %+v, want a single merged range ending at 10", got)
	}
}

func TestMergeTypedRangesDoesNotMergeAcrossTypes(t *testing.T) {
	// Adjacent and even touching, but different Joining_Type: must remain separate
	// records even though they're contiguous.
	in := []typedRange{
		{start: 0, end: 5, jtype: "L"},
		{start: 6, end: 10, jtype: "R"},
	}
	got := mergeTypedRanges(in)
	if len(got) != 2 {
		t.Fatalf("mergeTypedRanges = %+v, want 2 (different types must not merge)", got)
	}
}
