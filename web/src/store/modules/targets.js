import axios from "axios";

const state = () => {
  return {
    targets: []
  };
};

const getters = {
  getTargets: state => state.targets
};

const actions = {
  async fetchTargets({ commit, rootGetters }, limit) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.get(
        `http://localhost:8990/api/targets?limit=${limit}`,
        rootGetters["getAuthHeader"]
      );
      commit("setTargets", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  },

  async createTarget({ commit, rootGetters }, target) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.post(
        `http://localhost:8990/api/targets?name=${target.Name}&address=${target.Address}&retries=${target.Retries}&maxHops=${target.MaxHops}&timeout=${target.Timeout}`,
        "",
        rootGetters["getAuthHeader"]
      );
      commit("addTarget", response.data);
      return response.data;
    } catch (error) {
      console.log("Error caught: " + error);
      return false;
    }
  },

  async updateTarget({ commit, rootGetters }, target) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.put(
        `http://localhost:8990/api/targets`,
        target,
        rootGetters["getAuthHeader"]
      );
      commit("updateTarget", response.data);
      return response.data;
    } catch (error) {
      console.log("Error caught: " + error);
      return false;
    }
  },

  async deleteTarget({ commit, rootGetters }, targetID) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.delete(
        `http://localhost:8990/api/targets/${targetID}`,
        rootGetters["getAuthHeader"]
      );
      commit("removeTarget", response.data.ID);
      return response.data;
    } catch (error) {
      console.log("Error caught: " + error);
      return false;
    }
  }
};

const mutations = {
  setTargets: (state, targets) => (state.targets = targets),
  addTarget: (state, target) => state.targets.push(target),
  updateTarget: (state, target) => {
    state.targets = state.targets.map(el => (el.ID == target.ID ? target : el));
  },
  removeTarget: (state, targetID) =>
    (state.targets = state.targets.filter(el => el.ID != targetID))
};

export default {
  state,
  getters,
  actions,
  mutations
};
