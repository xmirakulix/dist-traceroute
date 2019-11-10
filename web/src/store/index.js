import Vuex from "vuex";
import Vue from "vue";
import Status from "./modules/status";
import Traces from "./modules/traces";

Vue.use(Vuex);

export default new Vuex.Store({
  modules: { Status, Traces }
});
