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
  },

  async createSlave({ commit, rootGetters }, slave) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.post(
        `http://localhost:8990/api/slaves?name=${slave.Name}&secret=${slave.Secret}`,
        "",
        rootGetters["getAuthHeader"]
      );
      commit("addSlave", response.data);
      return response.data;
    } catch (error) {
      console.log("Error caught: " + error);
      return false;
    }
  },

  async updateSlave({ commit, rootGetters }, slave) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.put(
        `http://localhost:8990/api/slaves`,
        slave,
        rootGetters["getAuthHeader"]
      );
      commit("updateSlave", response.data);
      return response.data;
    } catch (error) {
      console.log("Error caught: " + error);
      return false;
    }
  },

  async deleteSlave({ commit, rootGetters }, slaveId) {
    if (!rootGetters["isAuthorized"]) return;
    try {
      const response = await axios.delete(
        `http://localhost:8990/api/slaves/${slaveId}`,
        rootGetters["getAuthHeader"]
      );
      commit("removeSlave", response.data.ID);
      return response.data;
    } catch (error) {
      console.log("Error caught: " + error);
      return false;
    }
  }
};

const mutations = {
  setSlaves: (state, slaves) => (state.slaves = slaves),
  addSlave: (state, slave) => state.slaves.push(slave),
  updateSlave: (state, slave) => {
    state.slaves = state.slaves.map(el => (el.ID == slave.ID ? slave : el));
  },
  removeSlave: (state, slaveId) =>
    (state.slaves = state.slaves.filter(el => el.ID != slaveId))
};

export default {
  state,
  getters,
  actions,
  mutations
};
