# U64

Blazing Fast Unsigned 64-bit Integer Set in Go.

>- **High Performance:**
>
>   Read Optimized: 
>   Read is wait-free and Cache-friendly, which means only needs few atomic read sequentially.
>   
> 
>       Read could reach 500millions/second in a 20 logical core CPU.
>
>- **Low Overhead**
>
>   Although it can't compress digits, comparing other implementation which supports dynamically updating, it does save
>   memory because there is no pointer. Each key needs about 8.89Bytes, overhead is about 10%.
>
>- **Auto Scaling**
>
>   Expand automatically: When meet ErrNoSpace, it'll trigger expanding in async mode. The size will grow up to 2x as before.
>
>   Shrinking manually: Users could get usage of set and try to trigger shrinking or not. The set will do this job in async.
>   
>       Automatically shrinking needs extra information to make decision, it may bring unstable overhead(e.g. last modified need
>       get clock). So it's wiser to do such things in a higher level, because users may already have these helping information, 
>       there is no need to do the same jobs in set.

## Performance Tuning

### Aligned AVX Load(TODO)

[Reference](https://rigtorp.se/isatomic/)

## Limitation

1. The maximum size of set is 32Mi, but big enough for most cases. I set the limitation for avoiding unexpected memory
usage.

2. Only supports X86-64 platform.

3. It's better to use only one goroutine to update Set(Add/Remove), one goroutine is enough fast, and could avoid unnecessary cost of spin.

## Other Set Implementations

### Tree

We could make a Cache-friendly tree, but in Go, it's almost impossible to make an uint64 tree set without the pointer
overhead, so the space cost is at least 2x.

### Other Hash Set

The overhead is matter in normal hash set, because of the bucket linker. It's really hard to make it wait-free.

### Others

Actually the major issue is getting the balance of memory usage and performance. It's not an easy job, so I decide to
do it by myself.

## Mathematics

u64.Set has two tables for saving uint64 keys, there will be only one table could be written. If this one is full, try to 
add key into the other one.

The probability of both tables meet no space error is a serious problem, if the probability is high, the whole design
of the structure is unreliable.

The conclusion is: up to 5.42e-20

### The Proof

The mathematics proof is quite direct and simply:

We assume the load factor is 0.9 in the older table, then we make a 2x size table as new one, 
after transferring all keys into the new one, the new one's load factor will be 0.45.

The rate of each entry is not empty is 0.45.

The neighbour size is 64.

`P = 0.45^64 = 6.39e-25`

If the load factor is 0.5(which means the usage of the last table is 100%):

`P = 0.5^64 = 5.42e-20`
