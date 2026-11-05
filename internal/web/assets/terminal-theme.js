"use strict";
// The look every embedded terminal shares.
const TERMINAL_FONT = '"SF Mono", Menlo, Monaco, "JetBrains Mono", monospace';
function terminalTheme() {
  if (!window.matchMedia?.("(prefers-color-scheme: dark)").matches) return {
    background: "#fafbfc", foreground: "#262d38", cursor: "#262d38", cursorAccent: "#fafbfc",
    selectionBackground: "#adc8ec80", black: "#262d38", red: "#b53032", green: "#267a43",
    yellow: "#8b6300", blue: "#245cb5", magenta: "#80529a", cyan: "#227b86", white: "#d3d8e0",
    brightBlack: "#697586", brightRed: "#c83c3e", brightGreen: "#247c42", brightYellow: "#926d13",
    brightBlue: "#2868c9", brightMagenta: "#8b54aa", brightCyan: "#25818b", brightWhite: "#f3f5f8",
  };
  return {
    background: "#15181e", foreground: "#e4e7ed", cursor: "#e4e7ed",
    cursorAccent: "#15181e", selectionBackground: "#637da666",
    black: "#242933", red: "#f08080", green: "#a3be8c", yellow: "#ebcb8b",
    blue: "#81a9df", magenta: "#b99aca", cyan: "#88c0d0", white: "#d8dee9",
    brightBlack: "#687487", brightRed: "#ff9a9a", brightGreen: "#b8d5a0",
    brightYellow: "#f7dfa4", brightBlue: "#a5c7f4", brightMagenta: "#d2b5e2",
    brightCyan: "#a4dce8", brightWhite: "#ffffff",
  };
}
