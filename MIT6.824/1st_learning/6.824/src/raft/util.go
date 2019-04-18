package raft

import "log"

const (
	// used in the test file
	MARK           = false
	TEST_CONNECTED = false
	TEST_APPLY     = false
	SIMPLE_TEST    = false

	TIMER = false

	INIT                           = false
	LEADER_APPEND_LOG              = false
	LEADER_APPEN_LOG_REPLY_HANDLER = false
	SEND_HEARTBEAT_ROUTINE         = false
	APPLY_ENTRY_ROUTINE            = false

	CANVASS_VOTE = false

	REQUEST_VOTE_RPC = false
	APPEND_ENTRY_RPC = false

	FIND_MAJORITY_COMMIT_INDEX = false

	PERSIST = false
)

func DDEBUG(debug_state bool, format string, a ...interface{}) (n int, err error) {
	if debug_state == true {
		return DPrintf(format, a...)
	}
	return 0, nil
}

// Debugging
const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

func Max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}
func Min(a, b int) int {
	if a > b {
		return b
	} else {
		return a
	}
}

func findKthLargest(nums []int, k int) int {
	// sort nums[begin, end) with pivot = nums[begin]
	// find the Kth largest in [begin, end)
	var QuickSort func(nums []int, begin, end, k int) int
	QuickSort = func(nums []int, begin, end, k int) int {
		if begin+1 == end { // only one element
			return nums[begin]
		}
		pivot := nums[begin]
		cur := begin + 1
		i := begin   // nums[begin, i) is assumed <= pivot
		j := end - 1 // nums[j, end) is assumed >= pivot
		// algorithm stops when `i == j`
		for i != j {
			if nums[cur] > pivot {
				nums[cur], nums[j] = nums[j], nums[cur]
				j--
			} else {
				nums[cur], nums[i] = nums[i], nums[cur]
				i++
				cur++
			}
		}
		// nums[i,j] must equals to pivot
		big_part_size := end - j - 1
		if big_part_size+1 == k {
			return pivot
		}
		if big_part_size >= k {
			return QuickSort(nums, j+1, end, k)
		} else {
			return QuickSort(nums, begin, i, k-big_part_size-1)
		}
	}

	return QuickSort(nums, 0, len(nums), k)
}
