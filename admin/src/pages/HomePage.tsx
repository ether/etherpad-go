import {useStore} from "../store/store.ts";
import {useEffect, useMemo, useState} from "react";
import {InstalledPlugin, SearchParams} from "./Plugin.ts";
import {useDebounce} from "../utils/useDebounce.ts";
import {Trans, useTranslation} from "react-i18next";
import {SearchField} from "../components/SearchField.tsx";
import {determineSorting} from "../utils/sorting.ts";


export const HomePage = () => {
  const installedPlugins = useStore(state=>state.installedPlugins)
  const setInstalledPlugins = useStore(state=>state.setInstalledPlugins)
  const settingSocket = useStore(state=>state.settingSocket)
  const [searchParams, setSearchParams] = useState<SearchParams>({
    offset: 0,
    limit: 99999,
    sortBy: 'name',
    sortDir: 'asc',
    searchTerm: ''
  })

    const [searchTerm, setSearchTerm] = useState<string>('')
    const {t} = useTranslation()


    useEffect(() => {
        if(!settingSocket){
            return
        }

        settingSocket.on('results:installed', (data:{
            installed: InstalledPlugin[]
        })=>{
            setInstalledPlugins(data.installed)
        })

        settingSocket.on('results:updatable', (data: any) => {
          const newInstalledPlugins = useStore.getState().installedPlugins.map(plugin => {
            if (data.updatable.includes(plugin.name)) {
              return {
                ...plugin,
                updatable: true
              }
            }
            return plugin
          })
         setInstalledPlugins(newInstalledPlugins)
        })

        settingSocket.on('finished:install', () => {
            settingSocket!.emit('getInstalled');
        })

        settingSocket.on('finished:uninstall', () => {
            console.log("Finished uninstall")
        })


        // Reload on reconnect
        settingSocket.on('connect', ()=>{
            // Initial retrieval of installed plugins
            settingSocket.emit('getInstalled');
            settingSocket.emit('search', searchParams)
        })

        settingSocket.emit('getInstalled');

        // check for updates every 5mins
        const interval = setInterval(() => {
            settingSocket.emit('checkUpdates');
        }, 1000 * 60 * 5);

        return ()=>{
            clearInterval(interval)
        }
        }, [settingSocket]);


    useEffect(() => {
        if (!settingSocket) {
            return
        }
        settingSocket?.emit('search', searchParams)
        settingSocket!.on('results:searcherror', (data: {error: string}) => {
            console.log(data.error)
            useStore.getState().setToastState({
                open: true,
                title: "Error retrieving plugins",
                success: false
            })
        })
    }, [searchParams]);

    useDebounce(()=>{
        setSearchParams({
            ...searchParams,
            offset: 0,
            searchTerm: searchTerm
        })
    }, 500, [searchTerm])


    const activatedPlugins = useMemo(()=>{
        return installedPlugins.filter(p=>p.enabled)
    }, [installedPlugins])

    const deactivatedPlugins = useMemo(()=>{
        return installedPlugins.filter(p=>!p.enabled)
    }, [installedPlugins])


    return <div>
        <h1><Trans i18nKey="admin_plugins"/></h1>

        <h2><Trans i18nKey="admin_plugins.installed"/></h2>

        <table id="installed-plugins">
            <thead>
            <tr>
                <th><Trans i18nKey="admin_plugins.name"/></th>
                <th><Trans i18nKey="admin_plugins.description"/></th>
                <th><Trans i18nKey="admin_plugins.version"/></th>
            </tr>
            </thead>
            <tbody style={{overflow: 'auto'}}>
            {activatedPlugins.map((plugin) => {
                return <tr key={plugin.name}>
                    <td><a rel="noopener noreferrer" href={`https://npmjs.com/${plugin.name}`} target="_blank">{plugin.name}</a></td>
                    <td>{plugin.description}</td>
                    <td>{plugin.version}</td>
                        </tr>
                    })}
            </tbody>
        </table>


        <h2><Trans i18nKey="admin_plugins.available"/></h2>
        <SearchField onChange={v=>{setSearchTerm(v.target.value)}} placeholder={t('admin_plugins.available_search.placeholder')} value={searchTerm}/>

      <div className="table-container">
        <table id="available-plugins">
            <thead>
            <tr>
                <th className={determineSorting(searchParams.sortBy, searchParams.sortDir == "asc", 'name')} onClick={()=>{
                  setSearchParams({
                    ...searchParams,
                    sortBy: 'name',
                    sortDir: searchParams.sortDir === "asc"? "desc": "asc"
                  })
                }}>
                  <Trans i18nKey="admin_plugins.name" /></th>
                <th><Trans i18nKey="admin_plugins.description"/></th>
                <th className={determineSorting(searchParams.sortBy, searchParams.sortDir == "asc", 'version')} onClick={()=>{
                  setSearchParams({
                    ...searchParams,
                    sortBy: 'version',
                    sortDir: searchParams.sortDir === "asc"? "desc": "asc"
                  })
                }}><Trans i18nKey="admin_plugins.version"/></th>
            </tr>
            </thead>
            <tbody style={{overflow: 'auto'}}>
            {deactivatedPlugins.map((plugin) => {
                        return <tr key={plugin.name}>
                            <td><a rel="noopener noreferrer" href={`https://npmjs.com/${plugin.name}`} target="_blank">{plugin.name}</a></td>
                            <td>{plugin.description}</td>
                            <td>{plugin.version}</td>
                        </tr>
                    })
            }
            </tbody>
        </table>
      </div>
    </div>
}
