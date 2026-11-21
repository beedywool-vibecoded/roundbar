import js from '@eslint/js';
import prettier from 'eslint-config-prettier';
import vue from 'eslint-plugin-vue';
import globals from 'globals';
import tseslint from 'typescript-eslint';

/**
 * Flat config for ESLint 10, for the whole repo (web/ and server/).
 *
 * Pair it with .prettierrc.json: this file only lints, Prettier only formats, and
 * eslint-config-prettier (loaded last) turns off every ESLint rule that would
 * otherwise fight it. Editor: see .vscode/settings.json.
 */
const unusedVars = ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }];

export default [
    {
        ignores: [
            '**/node_modules/**',
            '**/dist/**',
            'build/**',
            'out/**',
            'coverage/**',
            'vendor/**',
            'server/data/**',
            '**/*.min.js',
        ],
    },

    js.configs.recommended,
    ...tseslint.configs.recommended,
    ...vue.configs['flat/recommended'],

    {
        files: ['**/*.js', '**/*.mjs', '**/*.ts'],
        languageOptions: {
            ecmaVersion: 'latest',
            sourceType: 'module',
        },
    },

    {
        // The PWA: browser modules, and its service worker
        files: ['web/**/*.ts', 'web/**/*.vue', 'web/**/*.js'],
        languageOptions: {
            globals: {
                ...globals.browser,
                ...globals.serviceworker,
            },
        },
    },

    {
        // .vue files: TypeScript inside <script setup lang="ts">
        files: ['**/*.vue'],
        languageOptions: {
            parserOptions: {
                parser: tseslint.parser,
                extraFileExtensions: ['.vue'],
            },
        },
    },

    {
        // The Express server, its tests, and the build tools
        files: ['server/**/*.ts', 'tools/**/*.mjs', 'tools/**/*.cjs', '**/*.config.ts', 'eslint.config.mjs'],
        languageOptions: {
            globals: {
                ...globals.node,
            },
        },
    },

    {
        // Node scripts, CommonJS files
        files: ['**/*.cjs'],
        languageOptions: {
            sourceType: 'commonjs',
        },
        rules: {
            '@typescript-eslint/no-require-imports': 'off',
        },
    },

    {
        rules: {
            // An unused `_next` (express error handlers need 4 params) is not dead code
            'no-unused-vars': 'off',
            '@typescript-eslint/no-unused-vars': unusedVars,
            // TypeScript already reports undefined names, with the real types
            'no-undef': 'off',
            // Shared components named in CLAUDE.md and used everywhere
            'vue/multi-word-component-names': ['error', { ignores: ['Collapse', 'Icon'] }],
        },
    },

    // Must stay last: turns off every rule that would fight Prettier
    prettier,
];
