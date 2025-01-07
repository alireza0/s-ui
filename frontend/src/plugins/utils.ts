import { i18n } from "@/locales"

type OBJ = {
  [key: string]: any
}

export const FindDiff = {
  Config(obj1: OBJ, obj2: OBJ): any[] {
    const differences: any[] = []

    if(!obj2){
      return [ { key: "all", obj: obj1 } ]
    }

    for (const key in obj1) {
      if (obj2.hasOwnProperty(key)) {
        const value1 = obj1[key]
        const value2 = obj2[key]

        if (Array.isArray(value1)){
          value1.forEach((v1,index) => {
            if(index >= value2.length){
              differences.push({key: key, action: "new", index: index, obj: v1})
            } else if(!this.deepCompare(v1,value2[index])) {
              differences.push({key: key, action: "edit", index: index, obj: v1})
            }
          })
        } else {
          if (!this.deepCompare(value1,value2)) {
            differences.push({ key: key, action: "set", obj: value1})
          }
        }
      } else {
        differences.push({ key: key, action: "set", obj: obj1[key]})
      }
    }

    return differences
  },
  ArrObj(value1: any[], value2: any[], key: string): any {
    const differences: any[] = []
    value1.forEach((v1,index) => {
      if(index >= value2.length) differences.push({key: key, action: "new", obj: v1})
      else if(!this.deepCompare(v1,value2[index])) differences.push({key: key, action: "edit", obj: v1})
    })
    return differences
  },
  Settings(obj1: OBJ, obj2: OBJ): any {
    const differences: any[] = []
    for (const key in obj1) {
      if (obj1[key] != obj2[key]) {
        differences.push({ key: key, action: "set", obj: obj1[key]})
      }
    }
    return differences
  },
  deepCompare(obj1: any, obj2: any): boolean {
    // Check if the types of both objects are the same
    if (typeof obj1 !== typeof obj2) {
      return false
    }
  
    // Check if both objects are arrays
    if (Array.isArray(obj1) && Array.isArray(obj2)) {
      if (obj1.length !== obj2.length) {
        return false
      }
  
      for (let i = 0; i < obj1.length; i++) {
        if (!this.deepCompare(obj1[i], obj2[i])) {
          return false
        }
      }
      return true
    }
  
    // Check if both objects are plain objects
    if (typeof obj1 === 'object' && typeof obj2 === 'object' && obj1 !== null && obj2 !== null) {
      const keys1 = Object.keys(obj1).filter(key => obj1[key] !== undefined)
      const keys2 = Object.keys(obj2).filter(key => obj2[key] !== undefined)
  
      if (keys1.length !== keys2.length) {
        return false
      }
  
      for (const key of keys1) {
        if (!keys2.includes(key) || !this.deepCompare(obj1[key], obj2[key])) {
          return false
        }
      }
      return true
    }
  
    // Check primitive values
    return obj1 === obj2
  }
}

const ONE_KB = 1024
const ONE_MB = ONE_KB * 1024
const ONE_GB = ONE_MB * 1024
const ONE_TB = ONE_GB * 1024
const ONE_PB = ONE_TB * 1024

export const HumanReadable = {
  sizeFormat(size:number, fix:number=2) {
    if (!size || size<0) return "-"
    if (size < ONE_KB) {
        return size.toFixed(0) + " " + i18n.global.t('stats.B')
    } else if (size < ONE_MB) {
        return (size / ONE_KB).toFixed(fix) + " " + i18n.global.t('stats.KB')
    } else if (size < ONE_GB) {
        return (size / ONE_MB).toFixed(fix) + " " + i18n.global.t('stats.MB')
    } else if (size < ONE_TB) {
        return (size / ONE_GB).toFixed(fix) + " " + i18n.global.t('stats.GB')
    } else if (size < ONE_PB) {
        return (size / ONE_TB).toFixed(fix) + " " + i18n.global.t('stats.TB')
    } else {
        return (size / ONE_PB).toFixed(fix) + " " + i18n.global.t('stats.PB')
    }
  },
  packetFormat(size:number, fix:number=2) {
    if (!size || size<0) return "-"
    if (size < 1000) {
        return size.toFixed(0) + " " + i18n.global.t('stats.p')
    } else if (size < 1000000) {
        return (size / 1000).toFixed(fix) + " " + i18n.global.t('stats.Kp')
    } else if (size < 1000000000) {
        return (size / 1000000).toFixed(fix) + " " + i18n.global.t('stats.Mp')
    } else {
        return (size / 1000000000).toFixed(fix) + " " + i18n.global.t('stats.Gp')
    }
  },
  formatSecond(second:number): string {
    if (!second || second<0) return "-"
    if (second < 60) {
        return second.toFixed(0) + i18n.global.t('date.s')
    } else if (second < 3600) {
        return (second / 60).toFixed(0) + i18n.global.t('date.m')
    } else if (second < 3600 * 24) {
        return (second / 3600).toFixed(0) + i18n.global.t('date.h')
    }
    const day = Math.floor(second / 3600 / 24)
    const remain = Math.floor((second/3600) - (day*24))
    return day + i18n.global.t('date.d') + (remain > 0 ? ' ' + remain + i18n.global.t('date.h') : '')
  },
  remainedDays(exp:number): string {
    if (exp == 0) return i18n.global.t('unlimited')
    const now = Date.now()/1000
    if (exp < now) return i18n.global.t('date.expired')
    return Math.floor((exp - now) / (3600*24)) + " " + i18n.global.t('date.d')
  }
}