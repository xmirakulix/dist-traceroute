import axios from "axios";

const state = () => {
  return {
    slaves: []
  };
};

const getters = {
  getSlaves: state => state.slaves
};

const actions = {
  async fetchSlaves({ commit, rootGetters }, limit) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.get(
        `http://localhost:8990/api/slaves?limit=${limit}`,
        rootGetters["getAuthHeader"]
      );
      commit("setSlaves", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  }
};

const mutations = {
  setSlaves: (state, slaves) => (state.slaves = slaves)
};

export default {
  state,
  getters,
  actions,
  mutations
};
