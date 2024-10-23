const seq = '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'.split('')

const RandomUtil = {
  randomIntRange(min: number, max: number): number {
    return parseInt((Math.random() * (max - min) + min).toString(), 10)
  },
  randomInt(n: number) {
    return this.randomIntRange(0, n)
  },
  randomSeq(count: number): string {
    let str = ''
    for (let i = 0; i < count; ++i) {
        str += seq[this.randomInt(62)]
    }
    return str
  },
  randomLowerAndNum(count: number): string {
    let str = ''
    for (let i = 0; i < count; ++i) {
        str += seq[this.randomInt(36)]
    }
    return str
  },
  randomUUID(): string {
    let d = new Date().getTime()
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
        let r = (d + Math.random() * 16) % 16 | 0
        d = Math.floor(d / 16)
        return (c === 'x' ? r : (r & 0x7 | 0x8)).toString(16)
    })
  },
  randomShadowsocksPassword(n: number): string {
    const array = new Uint8Array(n)
    window.crypto.getRandomValues(array)
    return btoa(String.fromCharCode(...array))
  },
  randomShortId(): string[] {
    let shortIds = new Array(24).fill('')
    for (var ii = 1; ii < 24; ii++) {
      for (var jj = 0; jj <= this.randomInt(7); jj++){
          let randomNum = this.randomInt(256)
          shortIds[ii] += ('0' + randomNum.toString(16)).slice(-2)
      }
  }
  return shortIds
  }
}

export default RandomUtil