const path = require('path')

module.exports = {
  target: 'electron-renderer',
  mode: 'production',
  entry: './src/index.ts',
  output: {
    path: path.resolve(__dirname, 'dist'),
    filename: 'index.js',
    libraryTarget: 'commonjs2',
  },
  resolve: {
    extensions: ['.ts', '.js'],
  },
  module: {
    rules: [
      { test: /\.ts$/, loader: 'ts-loader' },
      { test: /\.pug$/, loader: 'pug-loader' },
    ],
  },
  externals: {
    '@angular/animations': '@angular/animations',
    '@angular/common': '@angular/common',
    '@angular/core': '@angular/core',
    '@angular/forms': '@angular/forms',
    '@angular/platform-browser': '@angular/platform-browser',
    '@ng-bootstrap/ng-bootstrap': '@ng-bootstrap/ng-bootstrap',
    'tabby-core': 'tabby-core',
    'tabby-settings': 'tabby-settings',
    'tabby-terminal': 'tabby-terminal',
    rxjs: 'rxjs',
    'rxjs/operators': 'rxjs/operators',
  },
}
