import Vuex from "vuex";
import Vue from "vue";
import Status from "./modules/status";
import Traces from "./modules/traces";
import Auth from "./modules/auth";
import Slaves from "./modules/slaves";

Vue.use(Vuex);

export default new Vuex.Store({
  modules: { Status, Traces, Auth, Slaves }
});
