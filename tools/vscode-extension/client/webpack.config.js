const path = require('path');

module.exports = (env, argv) => {
  const isProduction = argv.mode === 'production';
  
  return {
    entry: {
      services: './src/webview/ServicesApp.tsx',
      domains: './src/webview/DomainsApp.tsx'
    },
    mode: isProduction ? 'production' : 'development',
    devtool: isProduction ? false : 'inline-source-map',
    output: {
      path: path.resolve(__dirname, 'out', 'webview'),
      filename: '[name].js',
      publicPath: isProduction ? '${webviewUri}/' : 'http://localhost:8080/',
    },
    resolve: {
      extensions: ['.ts', '.tsx', '.js', '.jsx'],
    },
    module: {
      rules: [
        {
          test: /\.tsx?$/,
          use: 'ts-loader',
          exclude: /node_modules/,
        },
        {
          test: /\.css$/,
          use: ['style-loader', 'css-loader'],
        },
      ],
    },
    externals: {
      vscode: 'commonjs vscode'
    },
  };
};