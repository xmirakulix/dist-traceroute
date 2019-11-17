import Vue from "vue";
import App from "@/App.vue";
import store from "@/store/store";
import vuetify from "@/plugins/vuetify";
import router from "@/router/router";
import axios from "axios";

axios.interceptors.response.use(
  function(response) {
    return response;
  },
  function(error) {
    if (error.response.status === 401 && router.currentRoute.path != "/login") {
      console.log(router.currentRoute);
      store.commit("unsetToken");
      router.push("/login");
    }
    return Promise.reject(error);
  }
);

Vue.config.productionTip = false;

new Vue({
  store,
  vuetify,
  router,
  render: h => h(App)
}).$mount("#app");
