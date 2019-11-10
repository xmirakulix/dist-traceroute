import axios from "axios";

const state = {
  status: {
    Uptime: "n/a",
    CurrentMasterConfig: {},
    LastSlaveConfigTime: "n/a",
    LastSlaveConfig: {}
  }
};

const getters = {
  getStatus: state => state.status
};

const actions = {
  async fetchStatus({ commit }) {
    try {
      const response = await axios.get("http://localhost:8990/api/status");
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
