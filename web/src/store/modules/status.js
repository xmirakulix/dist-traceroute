import axios from "axios";

const state = {
  status: {}
};

const getters = {
  status: state => state.status
};

const actions = {
  async fetchStatus({ commit }) {
    const response = await axios.get("http://localhost:8990/api/status");
    commit("setStatus", response.data);
  },

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
};

const mutations = {
  setStatus: (state, status) => (state.status = status),

  newSlave: (state, data) => console.log(data)
};

export default {
  state,
  getters,
  actions,
  mutations
};
