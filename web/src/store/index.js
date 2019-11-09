import Vuex from "vuex";
import Vue from "vue";
import overview from "./modules/overview";

Vue.use(Vuex);

export default new Vuex.Store({
  modules: { overview }
});
