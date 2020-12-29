#include "textflag.h"

#define key  R8
#define tbl  R9
#define atbl R10    // table aligned to 32Bytes.
#define left R11    // left bytes.
#define uleft R12   // unaligned left bytes.
#define keys Y0

// func containsAVX(key uint64, tbl *uint64, n int) bool
TEXT ·containsAVX(SB), NOSPLIT, $0

    MOVQ  k+0(FP), key
    MOVQ  t+8(FP), tbl
    MOVQ  tbl, R13
    ADDQ  $31, R13
    MOVQ  $31, atbl
    NOTQ  atbl
    ANDQ  R13, atbl
    MOVQ  n+16(FP), left
    MOVQ  tbl, uleft
    SUBQ  atbl, uleft
    JNZ   cmp8

aligned:
    VPBROADCASTQ k+0(FP), keys
    JMP    cmp32

loop32:
    ADDQ    $32, atbl
    SUBQ    $4, left
    JNAE    ret_false

cmp32:
    // 1. VPBBROADCASTQ 8byte->32byte 1
    // 2. VPCMPEQQ	2
    // 3. VPTEST 	3
    VMOVDQU (atbl), Y1
    VPCMPEQQ keys, Y1, Y2
    VPTEST  Y2, Y2
    JZ     loop32
    MOVB   $1, ret+24(FP)
    RET

loop8:
    ADDQ $8, tbl
    SUBQ $1, uleft
    JZ   aligned

cmp8:
    MOVQ  (tbl), R13
    CMPQ  R13, key
    JNE   loop8
    MOVB  $1, ret+24(FP)
    RET

ret_false:
    MOVB $0, ret+24(FP)
    RET

TEXT ·alignTo(SB), NOSPLIT, $0
    MOVQ n+0(FP), R8
    ADDQ $31, R8
    MOVQ $31, R9
    NOTQ R9
    ANDQ R9, R8
    MOVQ R8, r+8(FP)
    RET
