
## 2024-04-24 - [HIGH] Argument Injection via External Tool Output (adb devices)
**Vulnerability:** External tool output (like from `adb devices`) was parsed and the serial numbers were passed directly back into another `exec.CommandContext` without validation. If an attacker could spoof the output of the first command, they could inject arbitrary arguments into the subsequent commands.
**Learning:** We must not blindly trust the output of external tools, especially if that output is used to construct further shell commands or arguments. The lack of validation allowed for argument injection vulnerabilities.
**Prevention:** Always validate data coming from external processes using strict allowlists (e.g., regex `^[a-zA-Z0-9._:/\-]+$`) before using that data in subsequent commands.
