import React from 'react';
import ReactDOM from 'react-dom/client';
import { ServicesView } from './components/ServicesView';

// VS Code webview API
declare global {
  interface Window {
    acquireVsCodeApi: () => any;
  }
}

const vscode = window.acquireVsCodeApi();

// Render the React app
const root = ReactDOM.createRoot(document.getElementById('root')!);
root.render(<ServicesView vscode={vscode} />);