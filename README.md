# How to Run

### 1. Running the Program

To run the benchmark, navigate to the project directory and use the following command:

```bash
go run .
```

*This command automatically finds and runs all necessary `.go` files in the directory.*

### 2. Changing Variables

To change the number of iterations or the output size, open the `main.go` file and edit the variables inside the `main()` function as shown below.

```go
// main.go

func main() {
    // Edit these values for your test
    iterations := 100000 // Higher value for more consistent results
    dataSize := 256      // Size in bytes
    
    // ... rest of the code
}
