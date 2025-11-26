import {useEffect, useState} from 'react'
import './App.css'

import {NavLink, Outlet} from "react-router-dom";
import {LoadingScreen} from "./utils/LoadingScreen.tsx";
import {Trans, useTranslation} from "react-i18next";
import {Cable, Construction, Crown, NotepadText, Wrench, PhoneCall, LucideMenu} from "lucide-react";



export const App = () => {
  const {t} = useTranslation()
  const [sidebarOpen, setSidebarOpen] = useState<boolean>(true)

  useEffect(() => {
    document.title = t('admin.page-title')
  }, []);

  return <div id="wrapper" className={`${sidebarOpen ? '': 'closed' }`}>
    <LoadingScreen/>
    <div className="menu">
      <div className="inner-menu">
        <span>
                    <Crown width={40} height={40}/>
                    <h1>Etherpad</h1>
                </span>
        <ul onClick={()=>{
          if (window.innerWidth < 768) {
            setSidebarOpen(false)
          }
        }}>
          <li><NavLink to="/plugins"><Cable/><Trans i18nKey="admin_plugins"/></NavLink></li>
          <li><NavLink to={"/settings"}><Wrench/><Trans i18nKey="admin_settings"/></NavLink></li>
          <li><NavLink to={"/help"}> <Construction/> <Trans i18nKey="admin_plugins_info"/></NavLink></li>
          <li><NavLink to={"/pads"}><NotepadText/><Trans
            i18nKey="ep_admin_pads.ep_adminpads2_manage-pads"/></NavLink></li>
          <li><NavLink to={"/shout"}><PhoneCall/>Communication</NavLink></li>
        </ul>
      </div>
    </div>
      <button id="icon-button" onClick={() => {
        setSidebarOpen(!sidebarOpen)
      }}><LucideMenu/></button>
    <div className="innerwrapper">
      <Outlet/>
    </div>
  </div>
}

export default App
