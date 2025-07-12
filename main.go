package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/spf13/pflag"
)

var version = "v0.0.0"

// TODO: Useful to implement

type RingBuffer[T any] struct {
	Size  int
	Items []T
	Start int
	End   int
}

var _ = RingBuffer[int]{}

type application struct {
	filePath string
	Items    []*Item `json:"i,omitempty"`
	index    map[string]int
}

func NewApplication(config Config) *application {
	// Load the items from the file, which will be in the standard location:
	// - On Linux: $XDG_DATA_HOME/clip
	// - On macOS: $HOME/Library/Application Support/clip
	// - On Windows: %APPDATA%/clip

	filePath := os.Getenv("XDG_DATA_HOME")
	if filePath == "" {
		filePath = os.Getenv("HOME") + "/.local/share"
	}

	filePath += "/clip"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create the directory if it does not exist
		if err := os.MkdirAll(filePath, 0o755); err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}
	}

	filePath += "/data.json"
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close file: %v", err)
		}
	}()

	var app application
	if err := json.NewDecoder(file).Decode(&app); err != nil && err.Error() != "EOF" {
		log.Fatalf("Failed to decode JSON: %v", err)
	}
	app.filePath = filePath
	app.Reindex()

	return &app
}

func (app *application) Close() error {
	file, err := os.OpenFile(app.filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		log.Printf("Failed to open file for writing: %v", err)
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close file: %v", err)
		}
	}()

	if err := json.NewEncoder(file).Encode(app); err != nil {
		log.Printf("Failed to encode JSON: %v", err)
		return err
	}

	if err := file.Sync(); err != nil {
		log.Printf("Failed to sync file: %v", err)
		return err
	}

	return nil
}

type Config struct{}

type Item struct {
	Data string `json:"d,omitempty"`
	Hash string `json:"h,omitempty"`
}

func (app *application) hash(data string) string {
	data = strings.TrimSpace(data)
	hash := sha1.Sum([]byte(data))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func (app *application) Add(data string) {
	hash := app.hash(data)

	if idx, exists := app.index[hash]; exists && idx == len(app.Items)-1 {
		// Item already exists and is the latest, do nothing
		return
	} else if exists {
		// Remove it and re-add it to the end
		app.Remove(idx)
	}

	app.Items = append(app.Items, &Item{data, hash})
	app.index[hash] = len(app.Items) - 1
}

func (app *application) Get(index int) *Item {
	if index < 0 || index >= len(app.Items) {
		return nil
	}
	return app.Items[index]
}

func (app *application) Clear() {
	app.Items = nil
	app.index = make(map[string]int) // Reset index when deleting all items
}

func (app *application) Reindex() {
	app.index = make(map[string]int)
	for i, item := range app.Items {
		app.index[item.Hash] = i
	}
}

func (app *application) Remove(idx int) {
	if idx < 0 || idx >= len(app.Items) {
		return
	}

	if idx == 0 && len(app.Items) == 1 {
		app.Items = nil
		app.index = make(map[string]int) // Reset index if the last item is removed
		return
	}

	defer app.Reindex()

	if idx == 0 {
		app.Items = app.Items[1:]
		return
	}
	if idx == len(app.Items)-1 {
		app.Items = app.Items[:len(app.Items)-1]
		return
	}
	app.Items = append(app.Items[:idx], app.Items[idx+1:]...)
}

func (app *application) List() []*Item {
	return app.Items
}

type Flags struct {
	Operation Op
	Text      string // Positional argument for text input
	Silent    bool   // Flag to indicate if the text should be echoed back
	// FIX: We can't support negative indices in the flags directly, consider -P
	// for pasting negative index. We can't use this for deletes as it takes a
	// slice which can have mixed signs.
	PasteIndex    int
	DeleteIndices []int  // Slice of integers for delete indices
	ListArgs      [2]int // Range for listing items, first and last index
}

type Op int

const (
	OpHelp Op = iota
	OpVersion
	OpAdd
	OpPaste
	OpDelete
	OpDeleteAll
	OpList
)

func main() {
	pflag.Usage = func() {
		// Output to stderr
		fmt.Fprintln(os.Stderr, "Usage: clip [options|text]")
		fmt.Fprintln(os.Stderr, "Options:")
		pflag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nPositional argument:")
		fmt.Fprintln(os.Stderr, "  text: The text to add to the clipboard, if no text is provided, it will paste the last item from the clipboard.")
		fmt.Fprintln(os.Stderr, "\nExamples:")
		fmt.Fprintln(os.Stderr, "  clip 'Hello, World!'   # Adds 'Hello, World!' to the clipboard")
		fmt.Fprintln(os.Stderr, "  clip -s 'Hello, World!' # Adds 'Hello, World!' to the clipboard without echoing it back")
		fmt.Fprintln(os.Stderr, "  clip 		              # Pastes the latest item from the clipboard")
		fmt.Fprintln(os.Stderr, "  echo 'Hello, World!' | clip # Adds 'Hello, World!' to the clipboard from stdin")
		fmt.Fprintln(os.Stderr, "  clip -p=1              	# Pastes the item at index 1 from the clipboard")
		fmt.Fprintln(os.Stderr, "  clip -d=2	              # Deletes the item at index 2 from the clipboard")
		fmt.Fprintln(os.Stderr, "  clip -D                # Deletes all items from the clipboard")
		fmt.Fprintln(os.Stderr, "  clip -v                # Prints version information")
	}

	pflag.CommandLine.SortFlags = true
	pflag.BoolP("silent", "s", false, "Do not echo the text back to stdout after adding it to the clipboard")
	pflag.IntP("paste", "p", 0, "Paste the nth item from the clipboard; if n is not provided, paste the last item, negative values are interpreted as offsets from the end")
	pflag.IntSliceP("delete", "d", []int{0}, "Delete items from the clipboard; if n is not provided, delete the latest item, if multiple items are present delete them, negative values are interpreted as offsets from the end")
	pflag.BoolP("delete-all", "D", false, "Delete all items from the clipboard")
	pflag.IntSliceP("list", "l", []int{0, 0}, "List items in the clipboard; if no arguments are provided, list all items, if a single argument is provided [limit] it is used as a limit. If two arguments are provided [start] [end], they are used as the range to list items")
	pflag.BoolP("version", "v", false, "Print version information")

	// NoOptDefVal for flags
	pFlag := pflag.Lookup("paste")
	pFlag.NoOptDefVal = "0" // Default to pasting the last item if no argument is provided
	lFlag := pflag.Lookup("list")
	lFlag.NoOptDefVal = "0,0" // Default to listing all items if no arguments are provided
	dFlag := pflag.Lookup("delete")
	dFlag.NoOptDefVal = "0" // Default to deleting the latest item if no argument is provided
	sFlag := pflag.Lookup("silent")
	sFlag.Hidden = true // Hide the silent flag from the help output

	pflag.Parse()

	app := NewApplication(Config{})
	f, err := app.parse(pflag.CommandLine)
	if err != nil {
		pflag.Usage()
		os.Exit(1)
	}

	close := func() {
		if err := app.Close(); err != nil {
			log.Printf("Error closing application: %v", err)
		}
	}
	defer close()

	if err := app.handle(f); err != nil {
		if !errors.Is(err, pflag.ErrHelp) {
			log.Println(err.Error())
		}
		pflag.Usage()
		close()
		os.Exit(1)
	}
}

func (app *application) handle(flags Flags) error {
	switch flags.Operation {
	case OpHelp:
		pflag.Usage()
	case OpVersion:
		Outln(version)
	case OpAdd:
		if flags.Text == "" {
			return fmt.Errorf("no text provided to add to the clipboard")
		}
		app.Add(flags.Text)
		if !flags.Silent {
			Out(flags.Text)
		}
	case OpPaste:
		if len(app.Items) == 0 {
			return nil
		}
		idx, err := resolveIdx(flags.PasteIndex, len(app.Items))
		if err != nil {
			return err
		}

		item := app.Get(idx)
		if item == nil {
			return fmt.Errorf("item not found at index %d", idx)
		}

		// Bring this item to the front of the list
		// Unless it's already the latest item
		if idx != len(app.Items)-1 {
			app.Remove(idx)
			app.Add(item.Data) // Re-add it to the end of the list
		}

		// TODO: Allow adding a new line if they want it
		Out(item.Data)
	case OpDeleteAll:
		app.Clear()
	case OpDelete:
		var indices []int
		if len(flags.DeleteIndices) == 0 {
			indices = []int{0} // Default to deleting the latest item
		} else {
			indices = flags.DeleteIndices
		}

		// Sanitize indices to ensure they are within bounds
		for i, idx := range indices {
			idx, err := resolveIdx(idx, len(app.Items))
			if err != nil {
				return err
			}
			indices[i] = idx
		}

		// sort descending order to avoid index shifting issues
		slices.Sort(indices)
		slices.Reverse(indices)

		for _, i := range indices {
			app.Remove(i)
		}
	case OpList:
		if len(app.Items) == 0 {
			return nil // No items to list
		}

		start, end := flags.ListArgs[0], flags.ListArgs[1]
		if start == 0 && end == 0 {
			// List all items (in reverse order)
			for i := len(app.Items) - 1; i >= 0; i-- {
				item := app.Items[i]
				Outln(strings.ReplaceAll(item.Data, "\n", "\\n"))
			}
		} else {
			// IMPLEMENT: Limit and range listing
			panic("Not implemented yet")
		}
	default:
		return fmt.Errorf("unknown operation: %v", flags.Operation)
	}
	return nil
}

func resolveIdx(idx int, len int) (int, error) {
	if idx < 0 {
		idx = idx*-1 - 1
	} else {
		idx = len - idx - 1
	}

	if idx < 0 || idx >= len {
		return 0, fmt.Errorf("index %d out of bounds for length %d", idx, len)
	}

	return idx, nil
}

func (app *application) parse(flagset *pflag.FlagSet) (Flags, error) {
	var flags Flags
	flags.Operation = OpHelp // Default operation

	emptyArg0 := true
	if flagset.NArg() > 0 {
		// NOTE: No need to allow empty space to be copied
		// Instead we could utilize this to paste into an empty space in nvim.
		emptyArg0 = strings.TrimSpace(flagset.Arg(0)) == ""
		if !emptyArg0 {
			// Try again but unescaped
			emptyArg0 = strings.TrimSpace(strings.ReplaceAll(flagset.Arg(0), "\\n", "\n")) == ""
		}
	}

	if flagset.Changed("version") {
		v, err := flagset.GetBool("version")
		if err != nil {
			return flags, err
		}
		if v {
			flags.Operation = OpVersion
		}
	} else if flagset.Changed("delete-all") {
		d, err := flagset.GetBool("delete-all")
		if err != nil {
			return flags, err
		}
		if d {
			flags.Operation = OpDeleteAll
		}
	} else if flagset.Changed("delete") {
		indices, err := flagset.GetIntSlice("delete")
		if err != nil {
			return flags, err
		}
		if len(indices) == 0 || (len(indices) == 1 && indices[0] == 0) {
			flags.Operation = OpDelete
		} else {
			flags.Operation = OpDelete
			flags.DeleteIndices = indices
		}
	} else if flagset.Changed("list") {
		listArgs, err := flagset.GetIntSlice("list")
		if err != nil {
			return flags, err
		}
		if len(listArgs) == 0 {
			flags.Operation = OpList
		} else if len(listArgs) == 1 {
			flags.Operation = OpList
			flags.ListArgs[0] = listArgs[0]
		} else if len(listArgs) == 2 {
			flags.Operation = OpList
			flags.ListArgs[0] = listArgs[0]
			flags.ListArgs[1] = listArgs[1]
		} else {
			log.Println("Invalid number of arguments for list operation")
			return flags, pflag.ErrHelp
		}
	} else if flagset.Changed("paste") {
		flags.Operation = OpPaste
		paste, _ := flagset.GetInt("paste")
		flags.PasteIndex = paste
		// NOTE: Support piping back fzf of list output
		// Ex: `clip -l | fzf | clip -p`
		pipeInput, err := getPipeInput()
		if err != nil {
			return flags, fmt.Errorf("error reading piped input: %w", err)
		}

		if pipeInput != "" {
			// TODO: If an exact match isn't found this should do a prefix match.
			// TODO: In the future this should take into consideration list columns;
			// if / when we support truncating lists this will break unless we do
			// something to prevent that, like prefix matches, or adding an idx column
			// to the list output.

			// NOTE: Since we escape newlines in the list output, let's unescape them
			unescaped := strings.ReplaceAll(pipeInput, "\\n", "\n")
			hash := app.hash(unescaped)
			idx, exists := app.index[hash]
			if !exists && pipeInput != unescaped {
				hash = app.hash(pipeInput)
				idx, exists = app.index[hash]
			}
			if !exists {
				return flags, nil
			}
			if paste != 0 {
				// WARN: This ignores that the user could have explicitly set 0
				return flags, fmt.Errorf("piped input cannot be used when pasting an item by index")
			}

			// we need to invert the index (len - idx - 1)
			flags.PasteIndex = len(app.Items) - idx - 1
		}
	} else if flagset.NArg() == 1 && !emptyArg0 {
		flags.Operation = OpAdd
		flags.Text = flagset.Arg(0)
		if flagset.Changed("silent") {
			flags.Silent = true
		}
	} else if flagset.NArg() > 1 {
		log.Println("Invalid number of arguments")
		return flags, pflag.ErrHelp
	} else {
		// Now this could be either a piped input to a copy, otherwise it's a paste
		pipeInput, err := getPipeInput()
		if err != nil {
			return flags, err
		}

		if pipeInput != "" {
			flags.Operation = OpAdd
			flags.Text = pipeInput
			if flagset.Changed("silent") {
				flags.Silent = true
			}
		} else if emptyArg0 {
			flags.Operation = OpPaste
		} else {
			log.Println("Invalid operation, please provide a valid command or input")
			return flags, pflag.ErrHelp
		}
	}

	return flags, nil
}

func getPipeInput() (string, error) {
	// Wait for out to be done / flushed
	//if err := os.Stdout.Sync(); err != nil {
	//return "", fmt.Errorf("error flushing stdout: %w", err)
	//}

	info, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("error reading pipe status: %w", err)
	}
	if (info.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("error reading from pipe: %w", err)
		}

		// If trimming returns nothing, we return nothing
		if strings.TrimSpace(string(data)) == "" {
			return "", nil
		}

		// As a special case, if after we unescape newlines, and trim, we have
		// nothing, we return nothing
		if strings.TrimSpace(strings.ReplaceAll(string(data), "\\n", "\n")) == "" {
			return "", nil
		}

		return string(data), nil
	}
	return "", nil // No input from pipe
}

func Out(s string) {
	fmt.Print(s)
}

func Outf(format string, args ...any) {
	fmt.Printf(format, args...)
}

func Outln(s string) {
	fmt.Println(s)
}
