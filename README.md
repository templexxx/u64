# U64

Unsigned 64-bit Integer Set in Go.

>- **High Performance:**
>
>   Supports one write goroutine and multi read goroutine at the same time. Read is wait-free and Cache-friendly.
>   
>- **Rich Features:**
>
>   Search, Insert, Delete, List
>
>- **Saving memory:**
>
>   Although it can't compress digits, comparing other implementation which supports dynamically updating, it does save
>   memory because there is no pointer. Each key needs about 10Bytes, overhead is 25%.

## Limitation

1. The maximum size of set is 32Mi, big enough for most cases.

## Mathematics

u64.Set has two buckets for saving uint64 keys, there will be only one bucket could be written. If one is full, try to 
add key into the other one.

The probability of both buckets meet no space error is a serious problem, if the probability is high, the whole design
of the structure is unreliable.

The conclusion is: up to 5.47e-28

It's about 300μg/the Earth's Oceans.

### The Proof

The mathematics proof is quite direct and simply:

We assume the load factor is 0.75 in the older bucket which is the maximum load factor of one bucket, then we make a 2x size
bucket, after transferring all keys into the new one, the new one's load factor is 0.375 now.

The rate of each entry is not empty is 0.375.

The neighbour size is 64.

`P = 0.375^64 = 5.47e-28`

## Performance Tuning

### No Neighbour Bitmap

AVX atomic & extra 100% overhead
https://rigtorp.se/isatomic/

Compare benchmark: