package u64

import (
	"sync/atomic"
	"time"

	"github.com/templexxx/tsc"
)

// status struct(uint64):
// 64                                                                                  0
// <------------------------------------------------------------------------------------
// | is_running(1) | padding(1) | cycle1_rw(2) | cycle0_rw(2) | last_add(32) | cnt(26) |

// create status when New a Set.
func createStatus() uint64 {
	return 1<<63 | 3<<58 // set isRunning & cycle[0] read-write.
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
func (s *Set) couldScale() bool {

	sa := atomic.LoadUint64(&s.status)

	w, ar := s.getRW()
	if w != cycleNonExisted && ar == true {
		return true
	}
	return false
}

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
