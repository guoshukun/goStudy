// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

// There is a modified copy of this file in runtime/rwmutex.go.
// If you make any changes here, see if you should make them there.

// A RWMutex is a reader/writer mutual exclusion lock.
// The lock can be held by an arbitrary number of readers or a single writer.
// The zero value for a RWMutex is an unlocked mutex.
//
// A RWMutex must not be copied after first use.
//
// If a goroutine holds a RWMutex for reading and another goroutine might
// call Lock, no goroutine should expect to be able to acquire a read lock
// until the initial read lock is released. In particular, this prohibits
// recursive read locking. This is to ensure that the lock eventually becomes
// available; a blocked Lock call excludes new readers from acquiring the
// lock.
//
// In the terminology of the Go memory model,
// the n'th call to Unlock “synchronizes before” the m'th call to Lock
// for any n < m, just as for Mutex.
// For any call to RLock, there exists an n such that
// the n'th call to Unlock “synchronizes before” that call to RLock,
// and the corresponding call to RUnlock “synchronizes before”
// the n+1'th call to Lock.
type RWMutex struct {
    //Mutex互斥锁
	w           Mutex        // held if there are pending writers
    // 写锁信号量,用来唤醒或睡眠goroutine
	writerSem   uint32       // semaphore for writers to wait for completing readers
    // 读锁信号量,用来唤醒或睡眠goroutine
    //表示当前启用的读者数量，包括了所有正在临界区里面的读者或者
    //被写锁阻塞的等待进入临界区读者的数量。
    //相当于是当前调用了RLock函数并且还没调用RUnLock函数的读者的数量
	readerSem   uint32       // semaphore for readers to wait for completing writers
	// 读锁计数器
    readerCount atomic.Int32 // number of pending readers
	// 获取写锁时需要等待的读锁释放数量
    readerWait  atomic.Int32 // number of departing readers
}

//支持最多2^30个读
const rwmutexMaxReaders = 1 << 30

// Happens-before relationships are indicated to the race detector via:
// - Unlock  -> Lock:  readerSem
// - Unlock  -> RLock: readerSem
// - RUnlock -> Lock:  writerSem
//
// The methods below temporarily disable handling of race synchronization
// events in order to provide the more precise model above to the race
// detector.
//
// For example, atomic.AddInt32 in RLock should not appear to provide
// acquire-release semantics, which would incorrectly synchronize racing
// readers, thus potentially missing races.

// RLock locks rw for reading.
//
// It should not be used for recursive read locking; a blocked Lock
// call excludes new readers from acquiring the lock. See the
// documentation on the RWMutex type.
func (rw *RWMutex) RLock() {
    // 竞态检测
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
    // 每次goroutine获得读锁，readerCount+1
    // 1）如果写锁被获取，那么readerCount在 -rwmutexMaxReaders与0之间
    // 这时挂起获取读锁的goroutine。
    // 2）如果写锁未被获取，那么readerCount>=0，获取读锁,不阻塞。

    // 通过readerCount的正负判断读锁与写锁互斥,
    // 如果有写锁存在就挂起读锁的goroutine,多个读锁可以并行
	if rw.readerCount.Add(1) < 0 {
		// A writer is pending, wait for it.
		runtime_SemacquireRWMutexR(&rw.readerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
	}
}

// TryRLock tries to lock rw for reading and reports whether it succeeded.
//
// Note that while correct uses of TryRLock do exist, they are rare,
// and use of TryRLock is often a sign of a deeper problem
// in a particular use of mutexes.
func (rw *RWMutex) TryRLock() bool {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	for {
		c := rw.readerCount.Load()
		if c < 0 {
			if race.Enabled {
				race.Enable()
			}
			return false
		}
		if rw.readerCount.CompareAndSwap(c, c+1) {
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(&rw.readerSem))
			}
			return true
		}
	}
}

// RUnlock undoes a single RLock call;
// it does not affect other simultaneous readers.
// It is a run-time error if rw is not locked for reading
// on entry to RUnlock.
func (rw *RWMutex) RUnlock() {
    // 竞态检测
	if race.Enabled {
		_ = rw.w.state
		race.ReleaseMerge(unsafe.Pointer(&rw.writerSem))
		race.Disable()
	}
    // 释放读锁，将readerCount-1
    // 1）有读锁，没有写锁挂起，r>=0，释放锁成功
    // 2）有读锁，有写锁挂起 readerCount为[-rwmutexMaxReaders,0]; r=readerCount-1,<0
    // 3）没有读锁，没有写锁挂起 readerCount =0;r=readerCount-1,<0
    // 4）没有读锁，有写锁挂起。readerCount为-rwmutexMaxReaders; r=readerCount-1,<0
	if r := rw.readerCount.Add(-1); r < 0 {
		// Outlined slow-path to allow the fast-path to be inlined
        // 后面三种进入慢路径
		rw.rUnlockSlow(r)
	}
	if race.Enabled {
		race.Enable()
	}
}

func (rw *RWMutex) rUnlockSlow(r int32) {
    //经过RUnlock atomic.AddInt32(&rw.readerCount, 1)到这里已经没有读锁了
    // 但是r分上面三种情况下
    // 1）有读锁，没有写锁挂起，r>=0；进入下面逻辑
    // 2）没有读锁，没有写锁挂起 r+1=0;panic
    // 3）没有读锁，有写锁挂起 r+1 = -rwmutexMaxReaders;panic
	if r+1 == 0 || r+1 == -rwmutexMaxReaders {
		race.Enable()
		fatal("sync: RUnlock of unlocked RWMutex")
	}
	// A writer is pending.
    // 有读锁，有写锁挂起的这种情况
    // 更新获得写锁需要等待的读锁的数量
    // 当其==0时证明，所有等待的读锁全部释放掉
	if rw.readerWait.Add(-1) == 0 {
		// The last reader unblocks the writer.
        // 更新信号量，通知被挂起的写锁去获取锁
		runtime_Semrelease(&rw.writerSem, false, 1)
	}
}

// Lock locks rw for writing.
// If the lock is already locked for reading or writing,
// Lock blocks until the lock is available.
func (rw *RWMutex) Lock() {
    // 竞态检测
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	// First, resolve competition with other writers.
    //获得互斥锁，用来与其他goroutine互斥
	rw.w.Lock()
	// Announce to readers there is a pending writer.
    // 告诉其他来获取读锁操作的goroutine，已经有人获取了写锁
    // 此时readerCount应该介于-rwmutexMaxReaders～0之间
    // r为读锁数量
	r := rw.readerCount.Add(-rwmutexMaxReaders) + rwmutexMaxReaders
	// Wait for active readers.
    // 设置需要等待释放的读锁数量，
    // 如果有，则挂起当前写锁的goroutine，并监听写锁信号量
    // 如果没有，写加锁成功
	if r != 0 && rw.readerWait.Add(r) != 0 {
		runtime_SemacquireRWMutex(&rw.writerSem, false, 0)
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
		race.Acquire(unsafe.Pointer(&rw.writerSem))
	}
}

// TryLock tries to lock rw for writing and reports whether it succeeded.
//
// Note that while correct uses of TryLock do exist, they are rare,
// and use of TryLock is often a sign of a deeper problem
// in a particular use of mutexes.
func (rw *RWMutex) TryLock() bool {
	if race.Enabled {
		_ = rw.w.state
		race.Disable()
	}
	if !rw.w.TryLock() {
		if race.Enabled {
			race.Enable()
		}
		return false
	}
	if !rw.readerCount.CompareAndSwap(0, -rwmutexMaxReaders) {
		rw.w.Unlock()
		if race.Enabled {
			race.Enable()
		}
		return false
	}
	if race.Enabled {
		race.Enable()
		race.Acquire(unsafe.Pointer(&rw.readerSem))
		race.Acquire(unsafe.Pointer(&rw.writerSem))
	}
	return true
}

// Unlock unlocks rw for writing. It is a run-time error if rw is
// not locked for writing on entry to Unlock.
//
// As with Mutexes, a locked RWMutex is not associated with a particular
// goroutine. One goroutine may RLock (Lock) a RWMutex and then
// arrange for another goroutine to RUnlock (Unlock) it.
func (rw *RWMutex) Unlock() {
    // 竞态检测
	if race.Enabled {
		_ = rw.w.state
		race.Release(unsafe.Pointer(&rw.readerSem))
		race.Disable()
	}

	// Announce to readers there is no active writer.
    // 还原加锁时减去的那一部分readerCount
	r := rw.readerCount.Add(rwmutexMaxReaders)
    // 读锁数目超过了 最大允许数
	if r >= rwmutexMaxReaders {
		race.Enable()
		fatal("sync: Unlock of unlocked RWMutex")
	}
	// Unblock blocked readers, if any.
    // 唤醒获取读锁期间所有被阻塞的goroutine
	for i := 0; i < int(r); i++ {
		runtime_Semrelease(&rw.readerSem, false, 0)
	}
	// Allow other writers to proceed.
    // 释放互斥量资源，允许其他写操作。
	rw.w.Unlock()
	if race.Enabled {
		race.Enable()
	}
}

// RLocker returns a Locker interface that implements
// the Lock and Unlock methods by calling rw.RLock and rw.RUnlock.
func (rw *RWMutex) RLocker() Locker {
	return (*rlocker)(rw)
}

type rlocker RWMutex

func (r *rlocker) Lock()   { (*RWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*RWMutex)(r).RUnlock() }
