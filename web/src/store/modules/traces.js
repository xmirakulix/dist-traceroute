import axios from "axios";

const state = {
  traces: []
};

const getters = {
  getTraces: state => state.traces
};

const actions = {
  async fetchTraces({ commit }, limit) {
    try {
      const response = await axios.get(
        `http://localhost:8990/api/traces?_limit=${limit}`
      );
      commit("setTraces", response.data);
    } catch (error) {
      console.log("Error caught: " + error);
    }
  }
};

const mutations = {
  setTraces: (state, traces) => (state.traces = traces)
};

export default {
  state,
  getters,
  actions,
  mutations
};
