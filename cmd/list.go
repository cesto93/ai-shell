package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

type modelEntry struct {
	Name      string
	Provider  string
	TotalSize int64
	NewestMod int64
	FileCount int
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List downloaded models",
	Long: `List all GGUF models downloaded via the pull command.

Models are stored in ~/.ai-shell/models/. Companion files
(such as mmproj- projection files) are grouped under the
same model name.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		modelsDir, err := getModelsDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		entries, err := os.ReadDir(modelsDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No models found. Use 'ai-shell pull' to download a model.")
				return
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		groups := map[string]*modelEntry{}
		var groupKeys []string

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			name := modelNameFromFile(info.Name())
			entry, ok := groups[name]
			if !ok {
				entry = &modelEntry{Name: name, Provider: "llamacpp"}
				groups[name] = entry
				groupKeys = append(groupKeys, name)
			}
			entry.TotalSize += info.Size()
			entry.FileCount++
			if info.ModTime().Unix() > entry.NewestMod {
				entry.NewestMod = info.ModTime().Unix()
			}
		}

		if len(groups) == 0 {
			fmt.Println("No models found. Use 'ai-shell pull' to download a model.")
			return
		}

		sort.Slice(groupKeys, func(i, j int) bool {
			return groups[groupKeys[i]].NewestMod > groups[groupKeys[j]].NewestMod
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tPROVIDER\tSIZE\tFILES\tMODIFIED")
		fmt.Fprintln(w, "----\t--------\t----\t-----\t--------")

		for _, key := range groupKeys {
			g := groups[key]
			modStr := "(unknown)"
			if g.NewestMod > 0 {
				modStr = time.Unix(g.NewestMod, 0).Format("Jan 02 15:04")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
				g.Name,
				g.Provider,
				formatSize(g.TotalSize),
				g.FileCount,
				modStr,
			)
		}
		w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
