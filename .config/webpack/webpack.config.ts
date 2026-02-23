import path from 'path';
import { Configuration } from 'webpack';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import ReplaceInFileWebpackPlugin from 'replace-in-file-webpack-plugin';
import ForkTsCheckerWebpackPlugin from 'fork-ts-checker-webpack-plugin';

const config = (env: { production?: boolean }): Configuration => ({
  cache: {
    type: 'filesystem',
    buildDependencies: {
      config: [__filename],
    },
  },
  context: path.join(process.cwd(), 'src'),
  devtool: env.production ? 'source-map' : 'eval-source-map',
  entry: './module.ts',
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
    function (_context: string, request: string, callback: (err?: null, result?: string) => void) {
      const prefix = 'grafana/';
      if (request.indexOf(prefix) === 0) {
        return callback(undefined, request.substring(prefix.length));
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
          publicPath: `public/plugins/jira-datasource/`,
          outputPath: 'img/',
          filename: Boolean(env.production) ? '[hash][ext]' : '[name][ext]',
        },
      },
      {
        test: /\.(woff|woff2|eot|ttf|otf)(\?v=\d+\.\d+\.\d+)?$/,
        type: 'asset/resource',
        generator: {
          publicPath: `public/plugins/jira-datasource/`,
          outputPath: 'fonts/',
          filename: Boolean(env.production) ? '[hash][ext]' : '[name][ext]',
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
      async: Boolean(!env.production),
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
            replace: require(path.join(process.cwd(), 'package.json')).version,
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

export default config;
