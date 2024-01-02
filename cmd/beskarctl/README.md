# beskarctl
`beskarctl` is a command line tool for interacting with Beskar Artifact Registries.

## Installation
```
go install go.ciq.dev/beskar/cmd/beskarctl@latest
```

## Usage
`beskarctl` is very similar to `kubectl` in that it provide various subcommands for interacting with Beskar repositories. 
The following subcommands are available:
 ```
beskarctl yum <subcommand> [flags]
beskarctl static <subcommand> [flags]
beskarctl ostree <subcommand> [flags]
 ```
For more information on a specific subcommand, run `beskarctl <subcommand> --help`.

## Adding a new subcommand
Adding a new subcommand is fairly straightforward.  Feel free to use the existing subcommands as a template, e.g., 
`cmd/beskarctl/static/`.  The following steps should be followed:

1. Create a new file in `cmd/beskarctl/<subcommand>/root.go`.
2. Add a new `cobra.Command` to the `rootCmd` variable in `cmd/beskarctl/<subcommand>/root.go`.
3. Add an accessor function to `cmd/beskarctl/<subcommand>/root.go` that returns the new `cobra.Command`.
4. Register the new subcommand in `cmd/beskarctl/ctl/root.go` by calling the accessor function.

### Implementation Notes
- The `cobra.Command` you create should not be exported. Rather, your package should export an accessor function that 
returns the `cobra.Command`. The accessor function is your chance to set up any flags or subcommands that your 
`cobra.Command` needs.  Please avoid the use of init functi
- helper functions are available for common values such as `--repo` and `--registry`. See `cmd/beskarctl/ctl/helpers.go`