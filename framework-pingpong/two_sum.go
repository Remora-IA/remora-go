package main

func twoSumDemo() {
	nums := []int{1, 2, 3}
	target := 3
	_ = twoSum(nums, target)
}

func twoSum(nums []int, target int) []int {
	seen := map[int]int{}
	for i, n := range nums {
		if j, ok := seen[target-n]; ok {
			return []int{j, i}
		}
		seen[n] = i
	}
	return nil
}
