import axios from "axios";

const state = () => {
  return {
    status: {
      Uptime: "n/a",
      CurrentMasterConfig: {},
      LastSlaveConfigTime: "n/a",
      LastSlaveConfig: {}
    }
  };
};

const getters = {
  getStatus: state => state.status
};

const actions = {
  async fetchStatus({ commit, rootGetters }) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.get(
        "http://localhost:8990/api/status",
        rootGetters["getAuthHeader"]
      );
      commit("setStatus", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  }
};

const mutations = {
  setStatus: (state, status) => (state.status = status)
};

export default {
  state,
  getters,
  actions,
  mutations
};
