import React from 'react';
import { createRoot } from 'react-dom/client';
import { DomainsView } from './components/DomainsView';

declare global {
  interface Window {
    acquireVsCodeApi: () => any;
  }
}

const vscode = window.acquireVsCodeApi();

const App: React.FC = () => {
  return <DomainsView vscode={vscode} />;
};

const container = document.getElementById('root');
if (container) {
  const root = createRoot(container);
  root.render(<App />);
} else {
  // Root container not found - webview initialization error
}