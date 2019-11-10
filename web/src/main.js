import Vue from "vue";
import App from "./App.vue";
import store from "./store/store";
import vuetify from "./plugins/vuetify";
import router from "./router/router";

Vue.config.productionTip = false;

new Vue({
  store,
  vuetify,
  router,
  render: h => h(App)
}).$mount("#app");
