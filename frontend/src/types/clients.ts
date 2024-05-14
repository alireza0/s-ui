import RandomUtil from "@/plugins/randomUtil"

export interface Client {
  id?: number
	enable: boolean
	name: string
	config: string
	inbounds: string
  links: string
	volume: number
	expiry: number
  up: number
  down: number
  desc: string
}

const defaultClient: Client = {
  enable: true,
  name: "",
  config: "[]",
  inbounds: "",
  links: "[]",
  volume: 0,
  expiry: 0,
  up: 0,
  down: 0,
  desc: "",
}

type Config = {
  [key: string]: {
    name?: string
    username?: string
    [key: string]: any
  }
}

export function updateConfigs(configs: string, newUserName: string): string {
  const updatedConfigs: Config = JSON.parse(configs)

  for (const key in updatedConfigs) {
    if (updatedConfigs.hasOwnProperty(key)) {
      const config = updatedConfigs[key]
      if (config.hasOwnProperty("name")) {
        config.name = newUserName
      } else if (config.hasOwnProperty("username")) {
        config.username = newUserName
      }
    }
  }

  return JSON.stringify(updatedConfigs)
}

export function randomConfigs(user: string): Config {
  const mixedPassword = RandomUtil.randomSeq(10)
  const ssPassword = RandomUtil.randomShadowsocksPassword(32)
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
      password: ssPassword,
    },
    shadowtls: {
      name: user,
      password: ssPassword,
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
  return defaultObject
}
