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

  /*
  // eslint-disable-next-line no-unused-vars
  async addSlave({ commit }, title) {
    const response = await axios.post(
      "https://jsonplaceholder.typicode.com/todos",
      { title }
    );
    commit("newSlave", response.data);
  },

  async deleteSlave(id) {
    await axios.delete(`adasdasdasd/${id}`);
  }
  */
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
