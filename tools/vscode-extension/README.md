# Craft Language Extension

This extension provides syntax highlighting and language support for Craft Language files.

## Structure

```
.
├── client // Language Client
│   ├── src
│   │   ├── test // End to End tests for Language Client / Server
│   │   └── extension.ts // Language Client entry point
├── package.json // The extension manifest.
└── server // Language Server
    └── src
        └── server.ts // Language Server entry point
```

## Features

- Syntax highlighting for:
  - System definitions
  - Bounded contexts
  - Aggregates and components
  - Services and events
  - DDD patterns
  - Technology stacks
  - AWS services
- Code folding
- Comment toggling
- Bracket matching

## Installation

1. Clone this repository
2. Copy the `vscode-extension` folder
3. Run:
```bash
cd vscode-extension
npm install
vsce package
code --install-extension craft-0.0.1.vsix
```

## Development

- Run `npm install` in this folder. This installs all necessary npm modules in both the client and server folder
- Open VS Code on this folder.
- Press Ctrl+Shift+B to start compiling the client and server in [watch mode](https://code.visualstudio.com/docs/editor/tasks#:~:text=The%20first%20entry%20executes,the%20HelloWorld.js%20file.).
- Switch to the Run and Debug View in the Sidebar (Ctrl+Shift+D).
- Click to `Create a launch.json file`

Example of its contents:
```
// A launch configuration that compiles the extension and then opens it inside a new window
{
	"version": "0.2.0",
	"configurations": [
		{
			"type": "extensionHost",
			"request": "launch",
			"name": "Launch Client",
			"runtimeExecutable": "${execPath}",
			"args": ["--extensionDevelopmentPath=${workspaceRoot}"],
			"outFiles": [
				"${workspaceRoot}/client/out/**/*.js",
				"${workspaceRoot}/server/out/**/*.js"
			],
			"autoAttachChildProcesses": true,
			"preLaunchTask": {
				"type": "npm",
				"script": "watch"
			}
		}
	]
}
```

- Select `Launch Client` from the drop down (if it is not already).
- Press ▷ to run the launch config (F5).
- In the [Extension Development Host](https://code.visualstudio.com/api/get-started/your-first-extension#:~:text=Then%2C%20inside%20the%20editor%2C%20press%20F5.%20This%20will%20compile%20and%20run%20the%20extension%20in%20a%20new%20Extension%20Development%20Host%20window.) instance of VSCode, open a document in 'plain text' language mode.
  - Type `j` or `t` to see `Javascript` and `TypeScript` completion.
  - Enter text content such as `AAA aaa BBB`. The extension will emit diagnostics for all words in all-uppercase.

1. Open the extension in VS Code
2. Press F5 to run the extension in debug mode
3. Open a `.craft` file to test syntax highlighting

## Contributing

1. Fork the repository
2. Create a feature branch
3. Submit a pull request