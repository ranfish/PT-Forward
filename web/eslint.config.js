import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import pluginVue from 'eslint-plugin-vue'
import vueParser from 'vue-eslint-parser'
import globals from 'globals'

export default [
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...pluginVue.configs['flat/recommended'],
  {
    files: ['**/*.vue', '**/*.ts'],
    languageOptions: {
      parser: vueParser,
      parserOptions: {
        parser: tseslint.parser,
        project: './tsconfig.app.json',
        extraFileExtensions: ['.vue'],
      },
      globals: {
        ...globals.browser,
        ...globals.es2021,
        __APP_VERSION__: 'readonly',
      },
    },
    rules: {
      'vue/multi-word-component-names': 'off',
      'vue/singleline-html-element-content-newline': 'off',
      'vue/max-attributes-per-line': 'off',
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
      'no-console': ['warn', { allow: ['warn', 'error'] }],
      'no-empty': ['error', { allowEmptyCatch: true }],
      // §59.43: 禁止组件内私有字节格式化副本——必须 import @/utils/format 的
      // formatBytes/formatSpeed（单点维护，防 toFixed 位数/进制/后缀漂移）
      'no-restricted-syntax': ['error',
        {
          selector: "FunctionDeclaration[id.name=/^(formatSize|formatBytes|formatFileSize|formatDataSize)$/]",
          message: '禁止私有字节格式化函数：请 import { formatBytes } from "@/utils/format"（§59.43）',
        },
        {
          selector: "VariableDeclarator[id.name=/^(formatSize|formatBytes|formatFileSize|formatDataSize)$/][init.type='ArrowFunctionExpression']",
          message: '禁止私有字节格式化函数：请 import { formatBytes } from "@/utils/format"（§59.43）',
        },
        {
          selector: "Property[key.name=/^(formatSize|formatBytes|formatFileSize|formatDataSize)$/][value.type='ArrowFunctionExpression']",
          message: '禁止私有字节格式化函数：请 import { formatBytes } from "@/utils/format"（§59.43）',
        },
        // toFixed + 字节单位拼接形态（再发明兜底，窄匹配 GiB/MiB/KiB/TiB/GB/MB/KB/TB 字面量）
        {
          selector: "BinaryExpression[operator='+'] > CallExpression[callee.property.name='toFixed']",
          message: '字节格式化禁止内联 toFixed 拼接：请 import { formatBytes } from "@/utils/format"（§59.43，若非字节场景可用 eslint-disable-next-line 豁免）',
        },
      ],
    },
  },
  // §59.43 豁免：utils/format.ts 本体与测试文件不受限
  {
    files: ['src/utils/format.ts', 'src/utils/format.test.ts'],
    rules: {
      'no-restricted-syntax': 'off',
    },
  },
  {
    ignores: ['dist/**', 'node_modules/**', '*.d.ts'],
  },
]
