import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

// Expose APIs expected by existing index.html runtime.
window.Terminal = Terminal;
window.FitAddon = { FitAddon };
