import axios from "axios";

const state = {
  uptime: "100s"
};

const getters = {
  uptime: state => state.uptime
};

const actions = {
  async fetchUptime({ commit }) {
    const response = await axios.get(
      "https://jsonplaceholder.typicode.com/todos/1"
    );
    commit("setUptime", response.data.title);
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
  setUptime: (state, uptime) => (state.uptime = uptime),

  newSlave: (state, data) => console.log(data)
};

export default {
  state,
  getters,
  actions,
  mutations
};
