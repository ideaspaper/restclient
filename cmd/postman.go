package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ideaspaper/restclient/pkg/postman"
	"github.com/spf13/cobra"
)

var postmanCmd = &cobra.Command{
	Use:   "postman",
	Short: "Import and export Postman collections",
	Long: `Work with Postman Collection v2.1.0 format.

Examples:
  # Import a Postman collection to .http files
  restclient postman import collection.json

  # Import to a specific directory
  restclient postman import collection.json -o ./requests

  # Import to a single .http file
  restclient postman import collection.json --single-file

  # Export .http files to a Postman collection
  restclient postman export api.http

  # Export multiple .http files
  restclient postman export api.http users.http auth.http

  # Export with a custom collection name
  restclient postman export api.http --name "My API Collection"`,
}

var postmanImportCmd = &cobra.Command{
	Use:   "import <collection.json>",
	Short: "Import a Postman collection to .http file(s)",
	Long: `Import a Postman Collection v2.1.0 JSON file and convert it to .http file(s).

The importer supports:
  - Request items and folders
  - All HTTP methods
  - Headers, query parameters, and request body
  - URL-encoded and form-data bodies
  - Raw body (JSON, XML, etc.)
  - GraphQL requests
  - File references in body
  - Basic, Bearer, Digest, and AWS v4 authentication
  - Pre-request and test scripts (converted to .http format)
  - Collection and folder variables

Examples:
  # Import to current directory (creates Collection-Name/ folder)
  restclient postman import collection.json

  # Import to specific directory
  restclient postman import collection.json -o ./api-requests

  # Import as a single .http file (in current directory)
  restclient postman import collection.json --single-file

  # Import as a single .http file with custom path
  restclient postman import collection.json --single-file -o ./my-api.http

  # Import without variables
  restclient postman import collection.json --no-variables

  # Import without scripts
  restclient postman import collection.json --no-scripts`,
	Args: cobra.ExactArgs(1),
	RunE: runPostmanImport,
}

var postmanExportCmd = &cobra.Command{
	Use:   "export <file.http> [file2.http ...]",
	Short: "Export .http file(s) to a Postman collection",
	Long: `Export one or more .http files to a Postman Collection v2.1.0 JSON file.

The exporter supports:
  - Multiple requests per file
  - All HTTP methods
  - Headers, query parameters, and request body
  - URL-encoded and form-data bodies
  - Raw body with language hints (JSON, XML, etc.)
  - GraphQL requests
  - File references in body
  - Basic, Bearer, Digest, and AWS v4 authentication
  - Pre-request and test scripts
  - File-level variables (converted to collection variables)

Examples:
  # Export a single .http file
  restclient postman export api.http

  # Export multiple .http files
  restclient postman export api.http users.http auth.http

  # Export with custom collection name
  restclient postman export api.http --name "My API Collection"

  # Export with description
  restclient postman export api.http --description "API endpoints for my service"

  # Export to specific file
  restclient postman export api.http -o my-collection.json

  # Export without scripts
  restclient postman export api.http --no-scripts`,
	Args: cobra.MinimumNArgs(1),
	RunE: runPostmanExport,
}

// Import flags
var (
	importOutput      string
	importSingleFile  bool
	importNoVariables bool
	importNoScripts   bool
)

// Export flags
var (
	exportOutputFile     string
	exportCollectionName string
	exportDescription    string
	exportNoVariables    bool
	exportNoScripts      bool
	exportMinify         bool
)

func init() {
	rootCmd.AddCommand(postmanCmd)
	postmanCmd.AddCommand(postmanImportCmd)
	postmanCmd.AddCommand(postmanExportCmd)

	// Import flags
	postmanImportCmd.Flags().StringVarP(&importOutput, "output", "o", "", "Output path (directory for multi-file, file path for --single-file)")
	postmanImportCmd.Flags().BoolVar(&importSingleFile, "single-file", false, "Create a single .http file instead of multiple files")
	postmanImportCmd.Flags().BoolVar(&importNoVariables, "no-variables", false, "Don't include collection variables")
	postmanImportCmd.Flags().BoolVar(&importNoScripts, "no-scripts", false, "Don't include pre-request and test scripts")

	// Export flags
	postmanExportCmd.Flags().StringVarP(&exportOutputFile, "output", "o", "", "Output file path (default: <name>.postman_collection.json)")
	postmanExportCmd.Flags().StringVarP(&exportCollectionName, "name", "n", "", "Collection name (default: based on input filename)")
	postmanExportCmd.Flags().StringVarP(&exportDescription, "description", "d", "", "Collection description")
	postmanExportCmd.Flags().BoolVar(&exportNoVariables, "no-variables", false, "Don't include file variables as collection variables")
	postmanExportCmd.Flags().BoolVar(&exportNoScripts, "no-scripts", false, "Don't include pre-request and test scripts")
	postmanExportCmd.Flags().BoolVar(&exportMinify, "minify", false, "Output minified JSON (no formatting)")
}

func runPostmanImport(cmd *cobra.Command, args []string) error {
	collectionPath := args[0]

	// Check if file exists
	if _, err := os.Stat(collectionPath); os.IsNotExist(err) {
		return fmt.Errorf("collection file not found: %s", collectionPath)
	}

	// Determine output directory/file based on mode
	outputDir := "."
	outputFile := ""
	if importSingleFile {
		// In single-file mode, -o specifies the output file path
		outputFile = importOutput
	} else {
		// In multi-file mode, -o specifies the output directory
		if importOutput != "" {
			outputDir = importOutput
		}
	}

	opts := postman.ImportOptions{
		OutputDir:        outputDir,
		OutputFile:       outputFile,
		SingleFile:       importSingleFile,
		IncludeVariables: !importNoVariables,
		IncludeScripts:   !importNoScripts,
	}

	result, err := postman.Import(collectionPath, opts)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	// Print summary
	fmt.Printf("Successfully imported Postman collection\n")
	fmt.Printf("  Requests:  %d\n", result.RequestsCount)
	fmt.Printf("  Folders:   %d\n", result.FoldersCount)
	fmt.Printf("  Variables: %d\n", result.VariablesCount)
	fmt.Printf("\nFiles created:\n")
	for _, file := range result.FilesCreated {
		fmt.Printf("  - %s\n", file)
	}

	if len(result.Errors) > 0 {
		fmt.Printf("\nWarnings:\n")
		for _, err := range result.Errors {
			fmt.Printf("  - %s\n", err)
		}
	}

	return nil
}

func runPostmanExport(cmd *cobra.Command, args []string) error {
	// Validate input files
	for _, file := range args {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", file)
		}
		ext := strings.ToLower(filepath.Ext(file))
		if ext != ".http" && ext != ".rest" {
			return fmt.Errorf("invalid file type: %s (expected .http or .rest)", file)
		}
	}

	opts := postman.ExportOptions{
		CollectionName:        exportCollectionName,
		CollectionDescription: exportDescription,
		IncludeVariables:      !exportNoVariables,
		IncludeScripts:        !exportNoScripts,
		PrettyPrint:           !exportMinify,
	}

	// Determine output file
	outputFile := exportOutputFile
	if outputFile == "" {
		baseName := exportCollectionName
		if baseName == "" {
			// Use first file's name
			baseName = strings.TrimSuffix(filepath.Base(args[0]), filepath.Ext(args[0]))
		}
		// Sanitize name for filename
		baseName = strings.Map(func(r rune) rune {
			if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
				return '_'
			}
			return r
		}, baseName)
		outputFile = baseName + ".postman_collection.json"
	}

	result, err := postman.Export(args, outputFile, opts)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Print summary
	fmt.Printf("Successfully exported to Postman collection\n")
	fmt.Printf("  Requests:  %d\n", result.RequestsCount)
	fmt.Printf("  Variables: %d\n", result.VariablesCount)
	fmt.Printf("  Output:    %s\n", result.CollectionPath)

	return nil
}
