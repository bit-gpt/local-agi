package serverwallet

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func ParseAmount(amountStr string, decimals int) (*big.Int, error) {
	if strings.Contains(amountStr, ".") {
		parts := strings.Split(amountStr, ".")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid decimal format")
		}

		integerPart, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer part: %v", err)
		}

		decimalPart := parts[1]
		if len(decimalPart) > decimals {
			decimalPart = decimalPart[:decimals]
		}

		for len(decimalPart) < decimals {
			decimalPart += "0"
		}

		decimalValue, err := strconv.ParseInt(decimalPart, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid decimal part: %v", err)
		}

		multiplier := new(big.Int)
		multiplier.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

		total := new(big.Int)
		total.Mul(big.NewInt(integerPart), multiplier)
		total.Add(total, big.NewInt(decimalValue))

		return total, nil
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount format: %v", err)
	}

	multiplier := new(big.Int)
	multiplier.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	amountBig := big.NewFloat(amount)
	amountBig.Mul(amountBig, new(big.Float).SetInt(multiplier))

	result, _ := amountBig.Int(nil)
	return result, nil
}
