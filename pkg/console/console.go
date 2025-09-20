package console

import "fmt"

const (
	ColorGreen     = "\033[32m"
	ColorGray      = "\033[90m"
	ColorLightGray = "\033[37m"
	ColorReset     = "\033[0m"
)

func PrintBanner(version, ip, port string) {
	fmt.Printf("  %s%s Render %s%sv%s%s\n\n", ColorLightGray, "🔎", ColorGreen, ColorGray, version, ColorReset)
	fmt.Printf("  %s%sLocal:   %shttp://localhost:%s%s\n", ColorLightGray, "\u279c ", ColorReset, port, ColorReset)
	fmt.Printf("  %s%sNetwork: %shttp://%s:%s%s\n\n", ColorGray, "\u279c ", ColorReset, ip, port, ColorReset)
}
