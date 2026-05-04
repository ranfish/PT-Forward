import { createApp } from 'vue'
import Antd from 'ant-design-vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n from './composables/useI18n'
import 'ant-design-vue/dist/reset.css'
import './assets/styles/global.less'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(i18n)
app.use(Antd)
app.mount('#app')
