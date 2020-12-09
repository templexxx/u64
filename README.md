# U64

Unsigned 64-bit Integer Set in Go.

>- **High Performance:**
>
>   Read Optimized: 
>   Read is wait-free and Cache-friendly, which means only needs few atomic read sequentially.
>
>- **Low Overhead**
>
>   Although it can't compress digits, comparing other implementation which supports dynamically updating, it does save
>   memory because there is no pointer. Each key needs about 10Bytes, overhead is 25% (TODO test the overhead).
>
>- **Auto Scaling**
>
>   Expand automatically: When meet ErrNoSpace, it'll trigger expanding in async mode. The size will grow up to 2x as before.
>
>   Shrinking manually: Users could get usage of set and try to trigger shrinking or not. The set will decide to do this job in async.
>   
>       Automatically shrinking needs extra information to make decision, it may bring unstable overhead(e.g. last modified need
>       get clock). So it's wiser to do such things in high level, because users may already have these helping information, there
>       is no need to do the same jobs in set.

## Performance Tuning

### No Neighbour Bitmap

AVX atomic & extra 100% overhead
https://rigtorp.se/isatomic/

Compare benchmark:

### Using TSC register to get timestamp

## Limitation

1. The maximum size of set is 32Mi, but big enough for most cases. I set the limitation for avoiding unexpected memory
usage.

## Other Set Implementations

### Tree

We could make a Cache-friendly tree, but in Go, it's almost impossible to make an uint64 tree set without the pointer
overhead, so the space cost is at least 2x.

### Other Hash Set

The overhead is matter in normal hash set, because of the bucket linker. It's really hard to make it wait-free.

### Others

Actually the major issues are getting the balance of memory usage and performance. It's not a easy job, so I decide to
do it by myself.

## Mathematics

u64.Set has two buckets for saving uint64 keys, there will be only one bucket could be written. If one is full, try to 
add key into the other one.

The probability of both buckets meet no space error is a serious problem, if the probability is high, the whole design
of the structure is unreliable.

The conclusion is: up to 2.33e-14

### The Proof

The mathematics proof is quite direct and simply:

We assume the load factor is 0.75 in the older bucket which is the maximum load factor of one bucket, then we make a 2x size
bucket, after transferring all keys into the new one, the new one's load factor is 0.375 now.

The rate of each entry is not empty is 0.375.

The neighbour size is 32.

`P = 0.375^32 = 2.34e-18`

If the load factor is 0.5(which means the usage of the last table is 100%):

`P = 0.5^32 = 2.33e-14`

## Linkers

### Atomic 16Bytes