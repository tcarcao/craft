# Troubleshooting

Common issues and solutions for the Craft VSCode Extension. If you encounter a problem not covered here, please [report it on GitHub](https://github.com/tcarcao/craft-vscode-extension/issues).

## Syntax Highlighting Issues

### Syntax Highlighting Not Working

**Symptoms:** Your `.craft` file appears as plain text without colors

**[SCREENSHOT NEEDED: Language mode selector]**
*Show the language mode indicator in status bar and language selection dropdown*

**Solutions:**

1. **Check the language mode:**
   - Look at the status bar (bottom-right corner)
   - It should show "Craft Language"
   - If it shows something else, click it and select "Craft Language" from the dropdown

2. **Reload VS Code:**
   - Press `Ctrl+Shift+P` (or `Cmd+Shift+P` on Mac)
   - Type "Developer: Reload Window"
   - Press Enter

3. **Verify file extension:**
   - Ensure your file has the `.craft` extension
   - Rename if necessary: `myfile.txt` → `myfile.craft`

4. **Reinstall extension:**
   - Open Extensions view (`Ctrl+Shift+X`)
   - Find "Craft Arch Diagrams"
   - Click gear icon → Uninstall
   - Restart VS Code
   - Reinstall from marketplace

### Colors Look Wrong or Inconsistent

**Symptoms:** Syntax highlighting shows unexpected colors or some elements aren't highlighted

**Solutions:**

1. **Check your VS Code theme:**
   - Some themes may not support all semantic token types
   - Try switching to a different theme temporarily
   - File → Preferences → Color Theme

2. **Enable semantic highlighting:**
   - Open Settings (`Ctrl+,`)
   - Search for "semantic highlighting"
   - Ensure "Editor: Semantic Highlighting" is enabled

3. **Clear editor cache:**
   - Press `Ctrl+Shift+P` → "Developer: Reload Window"

## Diagram Generation Issues

### Diagrams Not Generating

**Symptoms:** Preview command runs but no diagram appears, or you see an error message

**[SCREENSHOT NEEDED: Error message when server is unreachable]**
*Show error toast notification when diagram server connection fails*

**Solutions:**

1. **Check server connection:**
   - Ensure your Craft server is running at the configured URL
   - Test by opening `http://localhost:8080` in a browser
   - You should see a response (not an error page)

2. **Verify server URL setting:**
   - Press `Ctrl+Shift+P` → "Craft: Open Settings"
   - Check that `craft.server.url` points to the correct address
   - Default is `http://localhost:8080`

3. **Check for DSL errors:**
   - Review your Craft code for syntax errors (red underlines)
   - Fix any validation errors before generating diagrams
   - Hover over errors to see detailed messages

4. **Increase timeout:**
   - For large architectures, increase `craft.server.timeout` in settings
   - Try 60000 (60 seconds) or higher
   - Navigate to Settings → Search "craft timeout"

5. **Check server logs:**
   - View → Output → Select "Craft Language Server"
   - Set `craft.logging.level` to `debug` in settings
   - Look for connection errors or server responses

### Preview Panel Shows Outdated Diagram

**Symptoms:** Diagram doesn't reflect recent code changes

**Solutions:**

1. **Save your file:**
   - Press `Ctrl+S` (or `Cmd+S` on Mac)
   - The extension processes saved content

2. **Regenerate the diagram:**
   - Press the preview shortcut again (`Ctrl+Shift+C` or `Ctrl+Shift+D`)
   - Or close the preview panel and reopen it

3. **Close old preview panels:**
   - You may have multiple preview panels open
   - Close all preview panels
   - Generate a fresh diagram

4. **Verify changes are valid:**
   - Check for syntax errors in your changes
   - Invalid syntax may prevent diagram updates

### Downloaded Diagrams Don't Match Preview

**Symptoms:** Downloaded diagram looks different from the preview

**Solutions:**

1. **Check diagram options:**
   - Verify settings in Domain Manager or Services view (gear icon)
   - Ensure mode (Detailed/Architecture, Transparent/Boundaries) is correct

2. **Verify selections:**
   - Check that the same elements are selected in sidebar
   - Selection changes affect the downloaded diagram

3. **Regenerate before downloading:**
   - Click Preview to regenerate
   - Then immediately download
   - This ensures preview and download are synchronized

## Sidebar View Issues

### Domain/Service Trees Empty

**Symptoms:** Sidebar views show "No domains found" or "No services found"

**[SCREENSHOT NEEDED: Empty sidebar view]**
*Show sidebar with "No domains found" message*

**Solutions:**

1. **Ensure valid file is open:**
   - You must have a `.craft` file open
   - File must contain valid `arch` and/or `services` blocks

2. **Check view mode:**
   - Switch between "Current File" and "Workspace" modes
   - Try both to see if domains appear

3. **Save your file:**
   - Press `Ctrl+S` to trigger a refresh
   - Sidebar views update on file save

4. **Refresh manually:**
   - Click the refresh icon in the toolbar
   - Or run "Craft: Refresh Domains" from Command Palette

5. **Check for parsing errors:**
   - Open Output panel (View → Output)
   - Select "Craft Language Server"
   - Look for parsing errors that might prevent tree building

### Selection Not Working in Sidebar

**Symptoms:** Checkboxes don't respond or selections get lost

**Solutions:**

1. **Refresh the view:**
   - Click the refresh button in the view toolbar
   - Or close and reopen the sidebar

2. **Save your file:**
   - Selections are preserved across refreshes if you save
   - Press `Ctrl+S`

3. **Reload the window:**
   - If issue persists: `Ctrl+Shift+P` → "Developer: Reload Window"

4. **Check for extension conflicts:**
   - Temporarily disable other extensions
   - Test if selections work
   - Re-enable extensions one by one to identify conflicts

### Cross-References Not Showing

**Symptoms:** "Also Involved In" section doesn't appear or is empty

**Solutions:**

1. **Verify use cases are defined:**
   - Subdomains must be referenced in multiple use cases
   - Check that your use cases actually reference the subdomain

2. **Refresh the domain tree:**
   - Click refresh icon
   - Save file to trigger update

3. **Expand the subdomain:**
   - Click the arrow/chevron next to the subdomain
   - Cross-references appear as child items

## Language Server Issues

### Auto-completion Not Working

**Symptoms:** No suggestions when typing, or `Ctrl+Space` does nothing

**[SCREENSHOT NEEDED: Output panel with Language Server logs]**
*Show Output panel with "Craft Language Server" selected displaying debug logs*

**Solutions:**

1. **Check language server status:**
   - View → Output → Select "Craft Language Server"
   - Look for "Language server started" message
   - Check for any error messages

2. **Enable semantic highlighting:**
   - File → Preferences → Settings
   - Search for "semantic highlighting"
   - Ensure "Editor: Semantic Highlighting" is enabled

3. **Reload the window:**
   - `Ctrl+Shift+P` → "Developer: Reload Window"

4. **Manually trigger completion:**
   - Press `Ctrl+Space` (or `Cmd+Space` on Mac)
   - Try typing more characters before triggering

5. **Check language mode:**
   - Ensure status bar shows "Craft Language"
   - If not, select it from the language dropdown

### Validation Errors Not Appearing

**Symptoms:** Syntax errors aren't highlighted in red

**Solutions:**

1. **Check that file is saved:**
   - Some validation happens on save
   - Press `Ctrl+S`

2. **Look for problems panel:**
   - View → Problems (`Ctrl+Shift+M`)
   - Errors may be listed here instead of inline

3. **Verify language server:**
   - View → Output → "Craft Language Server"
   - Check that server is running and processing files

4. **Reload the window:**
   - `Ctrl+Shift+P` → "Developer: Reload Window"

### Code Formatting Not Working

**Symptoms:** `Shift+Alt+F` doesn't format the document, or formatting looks wrong

**Solutions:**

1. **Check default formatter:**
   - Right-click in the editor
   - Select "Format Document With..."
   - Choose "Craft Language" as the formatter

2. **Verify language mode:**
   - Ensure file is recognized as Craft Language
   - Check status bar (bottom-right)

3. **Look for formatting errors:**
   - Some syntax errors prevent formatting
   - Fix validation errors first

4. **Try command palette:**
   - `Ctrl+Shift+P` → "Format Document"
   - See if an error message appears

## Performance Issues

### Extension Slow with Large Files

**Symptoms:** Extension is sluggish, syntax highlighting delays, slow diagram generation

**Solutions:**

1. **Split large files:**
   - Break architecture into smaller, domain-focused files
   - Use multi-file support to combine as needed

2. **Use "Current File" view:**
   - Switch from "Workspace" to "Current File" mode in sidebar
   - Reduces number of items to process

3. **Close preview panels:**
   - Close diagram previews when not needed
   - Preview panels consume resources

4. **Reduce logging level:**
   - Settings → Search "craft logging"
   - Set to `warn` or `error` instead of `debug`

5. **Increase server timeout:**
   - Settings → Search "craft timeout"
   - Set to higher value (e.g., 60000 for 60 seconds)

6. **Restart VS Code:**
   - Close all files
   - Restart VS Code to clear caches

## Multi-file Project Issues

### Workspace View Not Showing All Files

**Symptoms:** Some `.craft` files don't appear in Workspace view

**Solutions:**

1. **Check file locations:**
   - Ensure files are within the VS Code workspace
   - Files outside workspace aren't scanned

2. **Verify file extensions:**
   - All files must have `.craft` extension
   - Check for typos (`.craf`, `.Craft`, etc.)

3. **Refresh views:**
   - Click refresh icon in sidebar
   - Or reload window: `Ctrl+Shift+P` → "Developer: Reload Window"

4. **Save all files:**
   - File → Save All
   - Trigger workspace rescan

### Cross-File References Not Working

**Symptoms:** Domains from other files aren't recognized

**Solutions:**

1. **Ensure files are in workspace:**
   - All referenced files must be in the same VS Code workspace
   - Add folders to workspace if needed: File → Add Folder to Workspace

2. **Check domain names:**
   - Domain names must match exactly across files
   - Names are case-sensitive

3. **Reload workspace:**
   - Close and reopen VS Code
   - Or reload window: `Ctrl+Shift+P` → "Developer: Reload Window"

## Installation Issues

### Extension Not Installing from Marketplace

**Symptoms:** Install button doesn't work, or installation fails

**Solutions:**

1. **Check VS Code version:**
   - Ensure you have VS Code 1.75.0 or later
   - Help → About → Check version

2. **Update VS Code:**
   - Download latest version from https://code.visualstudio.com

3. **Try command line installation:**
   ```bash
   code --install-extension tcarcao.craft-arch-diagrams
   ```

4. **Install from VSIX:**
   - Download from [GitHub Releases](https://github.com/tcarcao/craft-vscode-extension/releases)
   - Install via "Extensions: Install from VSIX..."

5. **Check network connection:**
   - Marketplace requires internet access
   - Check firewall/proxy settings

## Getting More Help

### Enable Debug Logging

For detailed troubleshooting information:

1. Open Settings (`Ctrl+,`)
2. Search for "craft logging level"
3. Set to `debug` or `trace`
4. View → Output → Select "Craft Language Server"
5. Try reproducing the issue
6. Check the Output panel for detailed logs

### Check Extension Version

To verify you have the latest version:

1. Open Extensions view (`Ctrl+Shift+X`)
2. Search for "Craft Arch Diagrams"
3. Check version number
4. Click "..." menu → "Check for Extension Updates"

### Report an Issue

If you can't resolve the issue:

1. Gather information:
   - Extension version
   - VS Code version
   - Operating system
   - Error messages or screenshots
   - Output panel logs (with debug level enabled)

2. Visit [GitHub Issues](https://github.com/tcarcao/craft-vscode-extension/issues)

3. Search for existing issues first

4. Create a new issue with:
   - Clear description of the problem
   - Steps to reproduce
   - Expected vs actual behavior
   - All gathered information

## Common Error Messages

### "Cannot connect to Craft server"

**Cause:** Diagram server is not running or unreachable

**Solution:**
- Start the Craft server: See [Installation Guide](/guide/installation)
- Verify server URL in settings matches where server is running
- Check firewall settings

### "Request timeout"

**Cause:** Diagram generation took longer than configured timeout

**Solution:**
- Increase timeout in settings: `craft.server.timeout`
- Simplify your architecture (fewer services/domains)
- Check server performance

### "Invalid Craft syntax"

**Cause:** DSL syntax errors in your file

**Solution:**
- Look for red underlines in your code
- Hover over errors for details
- Fix syntax errors before generating diagrams

### "No domains found"

**Cause:** File doesn't contain valid domain definitions

**Solution:**
- Ensure file has `arch` block with domains
- Check syntax is correct
- Save file and refresh sidebar

## Next Steps

- [Browse all features](/extension/features)
- [Learn commands and shortcuts](/extension/commands)
- [Configure extension settings](/extension/configuration)
- [Report issues on GitHub](https://github.com/tcarcao/craft-vscode-extension/issues)
