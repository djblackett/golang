package main

import (
	"fmt"
	"math"
)

func printPrimes(max int) {
	for number := 2; number <= max; number++ {
		if number == 2 {
			fmt.Println(number)
			continue
		} else if number % 2 == 0 {
			continue
		}

		isPrime := true
		for factor := 3; float64(factor) <= math.Sqrt(float64(number)); factor += 2 {
			if number % factor == 0 {
				isPrime = false
				break
			}
		}
		if !isPrime {
			continue
		}
		fmt.Println(number)
		
	} 
}

// don't edit below this line

func test(max int) {
	fmt.Printf("Primes up to %v:\n", max)
	printPrimes(max)
	fmt.Println("===============================================================")
}

func main() {
	test(10)
	test(20)
	test(30)
}
