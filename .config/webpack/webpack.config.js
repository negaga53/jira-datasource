const path = require('path');
const CopyWebpackPlugin = require('copy-webpack-plugin');
const ReplaceInFileWebpackPlugin = require('replace-in-file-webpack-plugin');
const ForkTsCheckerWebpackPlugin = require('fork-ts-checker-webpack-plugin');
const packageJson = require('../../package.json');

/** @type {(env: Record<string, boolean>) => import('webpack').Configuration} */
module.exports = (env) => ({
  cache: {
    type: 'filesystem',
    buildDependencies: {
      config: [__filename],
    },
  },
  context: path.join(process.cwd(), 'src'),
  devtool: env.production ? 'source-map' : 'eval-source-map',
  entry: { module: './module.ts' },
  externals: [
    'lodash',
    'jquery',
    'moment',
    'slate',
    'emotion',
    '@emotion/react',
    '@emotion/css',
    'prismjs',
    'slate-plain-serializer',
    '@grafana/slate-react',
    'react',
    'react-dom',
    'react-redux',
    'redux',
    'rxjs',
    'd3',
    'angular',
    '@grafana/data',
    '@grafana/ui',
    '@grafana/runtime',
    '@grafana/e2e-selectors',
    function (data, callback) {
      const prefix = 'grafana/';
      const request = data.request || '';
      if (request.indexOf(prefix) === 0) {
        return callback(null, request.substring(prefix.length));
      }
      callback();
    },
  ],
  mode: env.production ? 'production' : 'development',
  module: {
    rules: [
      {
        exclude: /(node_modules)/,
        test: /\.[tj]sx?$/,
        use: {
          loader: 'swc-loader',
        },
      },
      {
        test: /\.css$/,
        use: ['style-loader', 'css-loader'],
      },
      {
        test: /\.s[ac]ss$/,
        use: ['style-loader', 'css-loader', 'sass-loader'],
      },
      {
        test: /\.(png|jpe?g|gif|svg)$/,
        type: 'asset/resource',
        generator: {
          publicPath: 'public/plugins/jira-datasource/',
          outputPath: 'img/',
          filename: env.production ? '[hash][ext]' : '[name][ext]',
        },
      },
      {
        test: /\.(woff|woff2|eot|ttf|otf)(\?v=\d+\.\d+\.\d+)?$/,
        type: 'asset/resource',
        generator: {
          publicPath: 'public/plugins/jira-datasource/',
          outputPath: 'fonts/',
          filename: env.production ? '[hash][ext]' : '[name][ext]',
        },
      },
    ],
  },
  output: {
    clean: {
      keep: /gpx_|plugin\.json|img\//,
    },
    filename: '[name].js',
    library: {
      type: 'amd',
    },
    path: path.resolve(process.cwd(), 'dist'),
    publicPath: '/',
    uniqueName: 'jira-datasource',
  },
  plugins: [
    new CopyWebpackPlugin({
      patterns: [
        { from: 'plugin.json', to: '.' },
        { from: '../README.md', to: '.', noErrorOnMissing: true },
        { from: '../LICENSE', to: '.', noErrorOnMissing: true },
        { from: 'img/**/*', to: '.', noErrorOnMissing: true },
        { from: 'dashboards/**/*', to: '.', noErrorOnMissing: true },
      ],
    }),
    new ForkTsCheckerWebpackPlugin({
      async: !env.production,
      issue: {
        include: [{ file: '**/*.{ts,tsx}' }],
      },
      typescript: { configFile: path.join(process.cwd(), 'tsconfig.json') },
    }),
    new ReplaceInFileWebpackPlugin([
      {
        dir: 'dist',
        files: ['plugin.json'],
        rules: [
          {
            search: '%TODAY%',
            replace: new Date().toISOString().substring(0, 10),
          },
          {
            search: '%VERSION%',
            replace: packageJson.version,
          },
        ],
      },
    ]),
  ],
  resolve: {
    extensions: ['.js', '.jsx', '.ts', '.tsx'],
    unsafeCache: true,
  },
});
