#include "textflag.h"

#define left BX
#define key  R8
#define tbl  DI
#define keys Y0

// func containsAVX(key uint64, tbl *uint64, n int) bool
TEXT ·containsAVX(SB), NOSPLIT, $0

    MOVQ  k+0(FP), key
    MOVQ  t+8(FP), tbl
    MOVQ  n+16(FP), left

    VPBROADCASTQ k+0(FP), keys

is_in_cacheLine:  // Split cache line will casue atomic load 256bit failed.
    CMPQ  left, $4
    JL    loop64
    MOVQ  tbl, R9
    SHRQ  $6, R9
    SHLQ  $6, R9
    ADDQ  $32, R9
    CMPQ  R9, tbl
    JL    loop64

loop256:
    // 1. VPBBROADCASTQ 8byte->32byte 1
    // 2. VPCMPEQQ	2
    // 3. VPTEST 	3
    VMOVDQU (tbl), Y1
    VPCMPEQQ keys, Y1, Y2
    ADDQ    $32, tbl
    SUBQ    $4, left
    VPTEST  Y2, Y2
    JZ      is_in_cacheLine
    SETEQ   ret+24(FP)
    RET

atomic64:
    ADDQ  $8, tbl
    SUBQ  $1, left
    JMP   is_in_cacheLine

loop64:
    CMPQ  left, $0
    JE    ret_false
    MOVQ  (tbl), R9
    CMPQ  R9, key
    JNE   atomic64
    SETEQ ret+24(FP)
    RET

ret_false:
    RET
