package main

import (
	"math"
)

func calculateStats(numbers []int) (min int, max int, avg float64, stdDev float64) {
    if len(numbers) == 0 {
        return 0, 0, 0, 0
    }

    min = numbers[0]
    max = numbers[0]
    sum := 0

    for _, number := range numbers {
        if number < min {
            min = number
        }
        if number > max {
            max = number
        }
        sum += number
    }

    avg = float64(sum) / float64(len(numbers))

    var sumOfSquares float64
    for _, number := range numbers {
        sumOfSquares += math.Pow(float64(number)-avg, 2)
    }

    stdDev = math.Sqrt(sumOfSquares / float64(len(numbers)))

    return min, max, avg, stdDev
}