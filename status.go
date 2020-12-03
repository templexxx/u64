package u64

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/templexxx/tsc"
)

// status struct(uint64):
// 64                                                                                                 58
// <---------------------------------------------------------------------------------------------------
// | is_running(1) | locked(1) | cycle1_obsoleted(1) | cycle1_w(1) | cycle0_obsoleted(1) |cycle0_w(1) |
// 58                       0
// <-------------------------
// | last_add(32) | cnt(26) |
//
// is_running: [63]
// locked: [62]
// cycle1_obsoleted: [61]
// cycle1_w: [60]
// cycle0_obsoleted: [59]
// cycle0_w: [58]
// last_add: [32, 58)
// cnt: [0,32)

// IsRunning returns Set is running or not.
func (s *Set) IsRunning() bool {
	sa := atomic.LoadUint64(&s.status)
	return (sa>>63)&1 == 1
}

// create status when New a Set.
func createStatus() uint64 {
	return 1<<63 | 1<<57 // set isRunning & cycle[0] is writable.
}

func (s *Set) lockOrRestart() bool {
	mu := new(sync.Mutex)
	if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
		return true
	}
	mu.Lock()
	mu.Unlock()
}

const cntMask = (1 << 26) - 1 // At most 1<<25.

// statusAdd update add info when new key added.
func (s *Set) statusAdd() {
	old := atomic.LoadUint64(&s.status)
	nv := old
	nv += 1 // Cnt is the lowest bits, just +1.
	ts := getTS()
	nv = (nv >> 58 << 58) | (uint64(ts) << 26) | (nv & cntMask)

	atomic.StoreUint64(&s.status, nv)
}

const (
	// epoch is an Unix time.
	// 2020-06-03T08:39:34.000+0800.
	epoch int64 = 1591144774
	// doom is the u64's max Unix time.
	// It will reach the end after 136 years from epoch.
	doom int64 = 5880040774 // epoch + 136 years (about 2^32 seconds).
	// maxTS is the u64's max timestamp.
	maxTS = uint32(doom - epoch)
)

// getTS gets u64 timestamp.
func getTS() uint32 {
	now := tsc.UnixNano()
	sec := now / int64(time.Second)
	ts := uint32(sec - epoch)
	if ts >= maxTS {
		panic("u64 met its doom")
	}
	return ts
}

// statusDel update add info when key has been deleted.
func (s *Set) statusDel() {
	atomic.AddUint64(&s.status, ^uint64(0))
}

// TODO should return now write idx, now cap
//func (s *Set) couldScale() bool {
//
//	sa := atomic.LoadUint64(&s.status)
//
//	w, ar := s.getRW()
//	if w != cycleNonExisted && ar == true {
//		return true
//	}
//	return false
//}

const (
	cycleNonExisted = 2 // cycle only has two slice, 2 is invalid.
	cycleBoth       = 3
)

// getRW gets Set write-only & read-only cycle indexes.
func (s *Set) getRW() (w, r uint8) {

	sa := atomic.LoadUint64(&s.status)
	rw0 := (sa >> 58) & 3
	rw1 := (sa >> 50) & 3
	if rw0 >= 2 {
		w = 0
		if rw1&1 == 1 {
			r = 1
		} else {
			r = cycleNonExisted
		}
	} else if rw1 >= 2 {
		w = 1
		if rw0&1 == 1 {
			r = 0
		} else {
			r = cycleNonExisted
		}
	} else {
		w = cycleNonExisted
		if rw0&1 == 1 {
			r = 0
		} else if rw1&1 == 1 {
			r = 1
		} else {
			r = cycleNonExisted
		}
	}

	return
}

// release releases obsoleted table.
func (s *Set) release(i int) {
	atomic.StorePointer(&s.cycle[i], nil)

	offset := i*2 + 58
	old := atomic.LoadUint64(&s.status)
	v := old | (1 << offset)
	atomic.StoreUint64(&s.status, v)
}

func (s *Set) exchangeRW() {

}
