import RandomUtil from "@/plugins/randomUtil"

export interface Link {
  type: "local" | "external" | "sub"
  remark?: string
  uri: string
}

export interface Client {
  id?: number
	enable: boolean
	name: string
	config?: Config
	inbounds: number[]
  links?: Link[]
	volume: number
	expiry: number
  up: number
  down: number
  desc: string
  group: string
}

const defaultClient: Client = {
  enable: true,
  name: "",
  config: {},
  inbounds: [],
  links: [],
  volume: 0,
  expiry: 0,
  up: 0,
  down: 0,
  desc: "",
  group: "",
}

type Config = {
  [key: string]: {
    name?: string
    username?: string
    [key: string]: any
  }
}

export function updateConfigs(configs: Config, newUserName: string): Config {
  for (const key in configs) {
    if (configs.hasOwnProperty(key)) {
      const config = configs[key]
      if (config.hasOwnProperty("name")) {
        config.name = newUserName
      } else if (config.hasOwnProperty("username")) {
        config.username = newUserName
      }
    }
  }
  return configs
}

export function shuffleConfigs(configs: Config, key?: string) {
  const keys = key ? [key] : Object.keys(configs)
  keys.forEach(k => {
    switch (k) {
      case "mixed":
      case "socks":
      case "http":
      case "anytls":
      case "trojan":
      case "naive":
      case "hysteria2":
        configs[k].password = RandomUtil.randomSeq(10)
        break
      case "shadowsocks":
        configs[k].password = RandomUtil.randomShadowsocksPassword(32)
        break
      case "shadowsocks16":
        configs[k].password = RandomUtil.randomShadowsocksPassword(16)
        break
      case "shadowtls":
        configs[k].password = RandomUtil.randomShadowsocksPassword(32)
        break
      case "hysteria":
        configs[k].auth_str = RandomUtil.randomSeq(10)
        break
      case "tuic":
        configs[k].password = RandomUtil.randomSeq(10)
        configs[k].uuid = RandomUtil.randomUUID()
        break
      case "vmess":
      case "vless":
        configs[k].uuid = RandomUtil.randomUUID()
        break
    }
  })
}

export function randomConfigs(user: string): Config {
  const mixedPassword = RandomUtil.randomSeq(10)
  const ssPassword16 = RandomUtil.randomShadowsocksPassword(16)
  const ssPassword32 = RandomUtil.randomShadowsocksPassword(32)
  const uuid = RandomUtil.randomUUID()
  return {
    mixed: {
      username: user,
      password: mixedPassword,
    },
    socks: {
      username: user,
      password: mixedPassword,
    },
    http: {
      username: user,
      password: mixedPassword,
    },
    shadowsocks: {
      name: user,
      password: ssPassword32,
    },
    shadowsocks16: {
      name: user,
      password: ssPassword16,
    },
    shadowtls: {
      name: user,
      password: ssPassword32,
    },
    vmess: {
      name: user,
      uuid: uuid,
      alterId: 0,
    },
    vless: {
      name: user,
      uuid: uuid,
      flow: "xtls-rprx-vision",
    },
    anytls: {
      name: user,
      password: mixedPassword,
    },
    trojan: {
      name: user,
      password: mixedPassword,
    },
    naive: {
      username: user,
      password: mixedPassword,
    },
    hysteria: {
      name: user,
      auth_str: mixedPassword,
    },
    tuic: {
      name: user,
      uuid: uuid,
      password: mixedPassword,
    },
    hysteria2: {
      name: user,
      password: mixedPassword,
    },
  }
}

export function createClient<T extends Client>(json?: Partial<T>): Client {
  defaultClient.name = RandomUtil.randomSeq(8)
  const defaultObject: Client = { ...defaultClient, ...(json || {}) }

  // Add missing config
  defaultObject.config = { ...randomConfigs(defaultObject.name), ...defaultObject.config }
  
  return defaultObject
}
