/** @type {import('@jest/types').Config} */
module.exports = {
  moduleNameMapper: {
    '\\.(css|scss|sass)$': '<rootDir>/.config/jest/styleMock.js',
    '\\.(jpg|jpeg|png|gif|svg)$': '<rootDir>/.config/jest/fileMock.js',
  },
  testEnvironment: 'jsdom',
  testMatch: ['<rootDir>/src/**/*.{spec,test}.{js,jsx,ts,tsx}'],
  transform: {
    '^.+\\.(t|j)sx?$': ['@swc/jest'],
  },
  transformIgnorePatterns: [
    'node_modules/(?!(@grafana|ol|geotiff|quick-lru|d3|d3-.*|internmap|delaunator|robust-predicates)/)',
  ],
};
